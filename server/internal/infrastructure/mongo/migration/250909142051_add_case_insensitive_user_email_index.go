package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveUserEmailIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")

	emailIndexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"email": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("email_case_insensitive_unique"),
	}

	_, err := col.Indexes().CreateOne(ctx, emailIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on user.email: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on user.email")
	return nil
}
