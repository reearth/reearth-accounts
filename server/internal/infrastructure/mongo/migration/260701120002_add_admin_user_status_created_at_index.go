package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddAdminUserStatusCreatedAtIndex creates a compound index on
// adminuser.{status, createdat} to list users of a given status in creation
// order efficiently.
func AddAdminUserStatusCreatedAtIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("adminuser")

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "status", Value: 1},
			{Key: "createdat", Value: 1},
		},
		Options: options.Index().SetName("adminuser_status_createdat"),
	}

	if _, err := col.Indexes().CreateOne(ctx, indexModel); err != nil {
		return fmt.Errorf("failed to create index on adminuser.status/createdat: %w", err)
	}
	fmt.Println("Created index on adminuser.status/createdat")
	return nil
}
