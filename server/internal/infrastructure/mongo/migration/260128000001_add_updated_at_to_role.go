package migration

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ApplyRoleUpdatedAtSchema(ctx context.Context, c DBClient) error {
	// Apply schema first to allow the updatedat field
	if err := ApplyCollectionSchemas(ctx, []string{"role"}, c); err != nil {
		return err
	}

	col := c.Database().Collection("role")

	cursor, err := col.Find(ctx, bson.M{"updatedat": bson.M{"$exists": false}})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		var updatedAt time.Time
		if id, ok := doc["_id"].(primitive.ObjectID); ok {
			updatedAt = id.Timestamp()
		} else {
			updatedAt = time.Now()
		}

		filter := bson.M{"_id": doc["_id"]}
		update := bson.M{"$set": bson.M{"updatedat": updatedAt}}

		if _, err := col.UpdateOne(ctx, filter, update); err != nil {
			return err
		}
	}

	return cursor.Err()
}
