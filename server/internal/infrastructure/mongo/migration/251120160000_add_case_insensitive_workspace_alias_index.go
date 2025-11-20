package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveWorkspaceAliasIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	// Scan for duplicate aliases (case-insensitive)
	duplicates, err := FindDuplicateWorkspaceAliases(ctx, col)
	if err != nil {
		return fmt.Errorf("failed to scan for duplicate workspace aliases: %w", err)
	}
	if len(duplicates) > 0 {
		fmt.Println("Duplicate workspace aliases found (case-insensitive):")
		for alias, ids := range duplicates {
			fmt.Printf("Alias: %s, Workspace IDs: %v\n", alias, ids)
		}
		return fmt.Errorf("cannot create index: duplicate aliases exist")
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
		return fmt.Errorf("failed to create unique index on workspace.alias: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on workspace.alias")
	return nil
}

// FindDuplicateWorkspaceAliases scans for duplicate workspace aliases (case-insensitive)
func FindDuplicateWorkspaceAliases(ctx context.Context, col *mongo.Collection) (map[string][]interface{}, error) {
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
