package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveUserAliasIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")

	aliasIndexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"alias": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale: "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("alias_case_insensitive_unique"),
	}
	_, err := col.Indexes().CreateOne(ctx, aliasIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on user.alias: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on user.alias")
	return nil
}
