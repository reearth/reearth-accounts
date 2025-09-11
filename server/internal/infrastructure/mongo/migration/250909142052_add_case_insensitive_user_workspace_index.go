package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddCaseInsensitiveUserWorkspaceIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")

	indexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"workspace": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("workspace_case_insensitive_unique"),
	}

	_, err := col.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on user.workspace: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on user.workspace")
	return nil
}
