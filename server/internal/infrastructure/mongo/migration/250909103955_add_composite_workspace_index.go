package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddWorkspaceAliasMembersCompositeUniqueIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	// Create composite unique index for alias and members
	compositeIndexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "alias", Value: 1},
			{Key: "members", Value: 1},
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("alias_members_case_insensitive_unique"),
	}

	_, err := col.Indexes().CreateOne(ctx, compositeIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create composite unique index on workspace.alias+members: %w", err)
	}
	fmt.Println("Created composite unique case-insensitive index on workspace.alias+members")
	return nil
}