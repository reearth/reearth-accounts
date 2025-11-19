package migration

import (
	"context"
	"fmt"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type workspaceMemberDoc struct {
	Role      string `bson:"role,omitempty"`
	InvitedBy string `bson:"invitedby,omitempty"`
	Disabled  bool   `bson:"disabled,omitempty"`
}

// convertToWorkspaceMemberDocument converts migration BSON types to the shared type
func convertToWorkspaceMemberDocument(members map[string]workspaceMemberDoc) map[string]mongodoc.WorkspaceMemberDocument {
	result := make(map[string]mongodoc.WorkspaceMemberDocument)
	for id, member := range members {
		result[id] = mongodoc.WorkspaceMemberDocument{
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
		}
	}
	return result
}

func AddWorkspaceMembersHash(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	// Count workspaces without members_hash field
	missingHashCount, err := col.CountDocuments(ctx, bson.M{"members_hash": bson.M{"$exists": false}})
	if err != nil {
		return fmt.Errorf("failed to count workspaces without members_hash: %w", err)
	}
	fmt.Printf("Workspaces without members_hash field: %d\n", missingHashCount)

	// Get all workspaces that don't have members_hash field
	cursor, err := col.Find(ctx, bson.M{"members_hash": bson.M{"$exists": false}})
	if err != nil {
		return fmt.Errorf("failed to find workspaces: %w", err)
	}
	defer cursor.Close(ctx)

	fmt.Printf("Starting to process workspaces without members_hash field...\n")
	updateCount := 0
	var bulkWrites []mongo.WriteModel

	for cursor.Next(ctx) {
		var doc struct {
			ID           primitive.ObjectID            `bson:"_id"`
			Members      map[string]workspaceMemberDoc `bson:"members"`
			Integrations map[string]workspaceMemberDoc `bson:"integrations"`
		}

		if err := cursor.Decode(&doc); err != nil {
			fmt.Printf("Warning: failed to decode workspace document: %v\n", err)
			continue
		}

		// Compute hash for this workspace using shared function
		members := convertToWorkspaceMemberDocument(doc.Members)
		integrations := convertToWorkspaceMemberDocument(doc.Integrations)
		membersHash, err := mongodoc.ComputeWorkspaceMembersHash(members, integrations)
		if err != nil {
			fmt.Printf("Warning: failed to compute members_hash for workspace %s: %v\n", doc.ID.Hex(), err)
			continue
		}

		// Prepare bulk update
		filter := bson.M{"_id": doc.ID}
		update := bson.M{"$set": bson.M{"members_hash": membersHash}}

		bulkWrites = append(bulkWrites, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
		updateCount++

		// Execute in batches of 1000
		if len(bulkWrites) >= 1000 {
			if _, err := col.BulkWrite(ctx, bulkWrites); err != nil {
				return fmt.Errorf("failed to execute bulk write: %w", err)
			}
			bulkWrites = bulkWrites[:0] // Reset slice
		}
	}

	// Execute remaining updates
	if len(bulkWrites) > 0 {
		if _, err := col.BulkWrite(ctx, bulkWrites); err != nil {
			return fmt.Errorf("failed to execute final bulk write: %w", err)
		}
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	fmt.Printf("Updated %d workspaces with members_hash field\n", updateCount)
	return nil
}

func ReplaceWorkspaceAliasMembersIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	// Drop the old problematic index
	_, err := col.Indexes().DropOne(ctx, "alias_members_case_insensitive_unique")
	if err != nil {
		fmt.Printf("Warning: failed to drop old index (might not exist): %v\n", err)
	} else {
		fmt.Println("Dropped old alias_members index")
	}

	// Create new compound index with alias and members_hash
	newIndexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "alias", Value: 1},
			{Key: "members_hash", Value: 1},
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2,
		}).SetUnique(true).SetName("alias_members_hash_case_insensitive_unique"),
	}

	_, err = col.Indexes().CreateOne(ctx, newIndexModel)
	if err != nil {
		return fmt.Errorf("failed to create new compound unique index on workspace.alias+members_hash: %w", err)
	}

	fmt.Println("Created new compound unique case-insensitive index on workspace.alias+members_hash")
	return nil
}
