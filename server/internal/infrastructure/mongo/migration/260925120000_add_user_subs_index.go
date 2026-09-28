package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddUserSubsIndexes creates the indexes backing User.FindBySub.
//
// FindBySub runs on every authenticated request and filters the user
// collection with
//
//	$or: [{subs: {$elemMatch: {$eq: sub}}}, {auth0sub: sub}, {auth0sublist: {$elemMatch: {$eq: sub}}}]
//
// MongoDB can only use indexes for an $or query when every clause is backed
// by an index; if any clause is unindexed the whole query falls back to a full
// collection scan. None of these fields were indexed, so every auth lookup
// scanned the entire user collection.
//
// subs gets a plain (multikey) ascending index. It is intentionally not
// unique because existing data may already contain duplicate subs.
// auth0sub and auth0sublist are legacy compat fields that are absent from most
// documents, so they get sparse indexes to keep them small.
//
// CreateIndexes is a no-op for indexes that already exist with an identical
// spec, so this migration is safe to re-run.
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
