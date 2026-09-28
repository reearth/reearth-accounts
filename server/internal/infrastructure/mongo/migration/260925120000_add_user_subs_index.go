package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddUserSubsIndexes(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("user")

	names, err := col.Indexes().CreateMany(ctx, userSubsIndexModels())
	if err != nil {
		return fmt.Errorf("failed to create sub indexes on user: %w", err)
	}
	fmt.Printf("Created indexes %q on user\n", names)
	return nil
}

func userSubsIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "subs", Value: 1}},
			Options: options.Index().SetName("subs"),
		},
		{
			Keys:    bson.D{{Key: "auth0sub", Value: 1}},
			Options: options.Index().SetName("auth0sub_sparse").SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "auth0sublist", Value: 1}},
			Options: options.Index().SetName("auth0sublist_sparse").SetSparse(true),
		},
	}
}
