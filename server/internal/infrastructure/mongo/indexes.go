package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateMany is a no-op for an existing identical index.
var indexes = map[string][]mongo.IndexModel{
	policyVersionsCollection: {{
		Keys:    bson.D{{Key: "type", Value: 1}, {Key: "version", Value: 1}},
		Options: options.Index().SetName("policy_versions_type_version").SetUnique(true),
	}},
}

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	for coll, models := range indexes {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes on %s: %w", coll, err)
		}
	}
	return nil
}
