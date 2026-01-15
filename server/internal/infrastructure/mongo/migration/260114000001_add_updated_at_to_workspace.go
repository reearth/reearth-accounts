package migration

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ApplyWorkspaceUpdatedAtSchema(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	cursor, err := col.Find(ctx, bson.M{"updatedAt": bson.M{"$exists": false}})
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
		update := bson.M{"$set": bson.M{"updatedAt": updatedAt}}

		if _, err := col.UpdateOne(ctx, filter, update); err != nil {
			return err
		}
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	return ApplyCollectionSchemas(ctx, []string{"workspace"}, c)
}
