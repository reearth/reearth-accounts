package migration

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ApplyPermittableUpdatedAtSchema(ctx context.Context, c DBClient) error {
	// Update schema validator first to allow the updatedat field
	if err := ApplyCollectionSchemas(ctx, []string{"permittable"}, c); err != nil {
		return err
	}

	col := c.Database().Collection("permittable")

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

	if err := cursor.Err(); err != nil {
		return err
	}

	return nil
}
