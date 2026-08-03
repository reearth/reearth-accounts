package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// AddWorkspaceMembersWildcardIndex creates a wildcard index over the
// workspace.members subdocument.
//
// FindByUser/FindByUserWithPagination filter on the dynamic field path
// "members.<userID>: {$exists: true}". Because members is stored as a map
// keyed by user ID, no regular index can target that path, so every by-user
// workspace lookup used to be a full collection scan. A wildcard index
// indexes every field under a subdocument regardless of key name (requires
// MongoDB 4.2+), which lets Mongo serve this exact "does key X exist under
// members" query without any change to the document shape or the query
// itself, and without needing a backfill since the index is built directly
// from the existing members maps.
func AddWorkspaceMembersWildcardIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	name, err := col.Indexes().CreateOne(ctx, workspaceMembersWildcardIndexModel())
	if err != nil {
		return fmt.Errorf("failed to create wildcard index on workspace.members: %w", err)
	}
	fmt.Printf("Created wildcard index %q on workspace.members\n", name)
	return nil
}

func workspaceMembersWildcardIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{{Key: "members.$**", Value: 1}},
	}
}
