package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveUserSubsIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")

	subsIndexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"subs": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("subs_case_insensitive_unique"),
	}

	_, err := col.Indexes().CreateOne(ctx, subsIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on user.subs: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on user.subs")
	return nil
}