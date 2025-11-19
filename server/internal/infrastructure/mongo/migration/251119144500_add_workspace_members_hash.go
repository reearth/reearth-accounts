package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type workspaceMemberData struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	InvitedBy string `json:"invited_by"`
	Disabled  bool   `json:"disabled"`
	Type      string `json:"type"` // "user" or "integration"
}

type workspaceMemberDoc struct {
	Role      string `bson:"role,omitempty"`
	InvitedBy string `bson:"invitedby,omitempty"`
	Disabled  bool   `bson:"disabled,omitempty"`
}

// computeMembersHashFromBSON creates a deterministic hash of members and integrations
func computeMembersHashFromBSON(members, integrations map[string]workspaceMemberDoc) string {
	var allMembers []workspaceMemberData

	// Add users
	for id, member := range members {
		allMembers = append(allMembers, workspaceMemberData{
			ID:        id,
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
			Type:      "user",
		})
	}

	// Add integrations
	for id, member := range integrations {
		allMembers = append(allMembers, workspaceMemberData{
			ID:        id,
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
			Type:      "integration",
		})
	}

	// Sort by ID for deterministic ordering
	sort.Slice(allMembers, func(i, j int) bool {
		return allMembers[i].ID < allMembers[j].ID
	})

	// Convert to JSON and hash
	jsonData, _ := json.Marshal(allMembers)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

func AddWorkspaceMembersHash(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("workspace")

	// First, let's see what workspaces exist
	totalCount, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to count workspaces: %w", err)
	}
	fmt.Printf("Total workspaces in collection: %d\n", totalCount)

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

		fmt.Printf("Processing workspace ID: %s, Members: %+v, Integrations: %+v\n", doc.ID.Hex(), doc.Members, doc.Integrations)

		// Compute hash for this workspace
		membersHash := computeMembersHashFromBSON(doc.Members, doc.Integrations)
		fmt.Printf("Computed hash for workspace %s: %s\n", doc.ID.Hex(), membersHash)

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
