package migration

import (
	"context"
	"fmt"

	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveUserAliasIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")
	colUser := c.Collection("user")

	duplicates, err := FindDuplicateUserAliases(ctx, col)
	if err != nil {
		return fmt.Errorf("failed to scan for duplicate user aliases: %w", err)
	}
	if len(duplicates) > 0 {
		fmt.Println("Duplicate user aliases found (case-insensitive):")
		for alias, ids := range duplicates {
			fmt.Printf("Alias: %s, User IDs: %v\n", alias, ids)
		}
		if err := GenerateNewAliasesForDuplicateUsers(ctx, colUser, duplicates); err != nil {
			return fmt.Errorf("failed to generate new aliases for duplicates: %w", err)
		}
		fmt.Println("Generated new random aliases for duplicate users.")
	}

	aliasIndexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"alias": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("alias_case_insensitive_unique"),
	}

	_, err = col.Indexes().CreateOne(ctx, aliasIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on user.alias: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on user.alias")
	return nil
}

// FindDuplicateUserAliases scans for duplicate user aliases (case-insensitive)
func FindDuplicateUserAliases(ctx context.Context, col *mongo.Collection) (map[string][]interface{}, error) {
	pipeline := []interface{}{
		map[string]interface{}{
			"$group": map[string]interface{}{
				"_id": map[string]interface{}{
					"$toLower": "$alias",
				},
				"ids":   map[string]interface{}{"$push": "$_id"},
				"count": map[string]interface{}{"$sum": 1},
			},
		},
		map[string]interface{}{
			"$match": map[string]interface{}{
				"count": map[string]interface{}{"$gt": 1},
			},
		},
	}

	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	duplicates := make(map[string][]interface{})
	for cursor.Next(ctx) {
		var result struct {
			ID    string        `bson:"_id"`
			IDs   []interface{} `bson:"ids"`
			Count int           `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		duplicates[result.ID] = result.IDs
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

// GenerateNewAliasesForDuplicateUsers assigns new random aliases to users with duplicate aliases
func GenerateNewAliasesForDuplicateUsers(ctx context.Context, col *mongox.Collection, duplicates map[string][]interface{}) error {
	var ids []string
	var newRows []interface{}

	for lowerAlias, userIDs := range duplicates {
		// Keep the first user with the original alias, change the rest
		for i, id := range userIDs {
			if i == 0 {
				fmt.Printf("Keeping user %v with original alias for: %s\n", id, lowerAlias)
				continue
			}

			var doc mongodoc.UserDocument
			filter := bson.M{"_id": id}
			err := col.Client().FindOne(ctx, filter).Decode(&doc)
			if err != nil {
				return fmt.Errorf("failed to find user with id %v: %w", id, err)
			}

			newAlias := fmt.Sprintf("%s-%s", lowerAlias, random.String(6, random.Lowercase))
			doc.Alias = newAlias

			ids = append(ids, doc.ID)
			newRows = append(newRows, doc)
			fmt.Printf("Prepared user %v with new alias: %s\n", doc.ID, newAlias)
		}
	}

	if len(ids) > 0 {
		if err := col.SaveAll(ctx, ids, newRows); err != nil {
			return fmt.Errorf("failed to bulk update aliases: %w", err)
		}
	}
	return nil
}
