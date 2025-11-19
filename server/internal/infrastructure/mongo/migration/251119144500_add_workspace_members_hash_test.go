package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAddWorkspaceMembersHash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongotest.Connect(t)(t)
	col := db.Collection("workspace")

	// Insert test workspace without members_hash
	testWorkspace := bson.M{
		"_id":   "workspace1",
		"alias": "test-workspace",
		"name":  "Test Workspace",
		"email": "test@example.com",
		"members": bson.M{
			"user1": bson.M{
				"role":      "owner",
				"invitedby": "user1",
				"disabled":  false,
			},
			"user2": bson.M{
				"role":      "reader",
				"invitedby": "user1",
				"disabled":  false,
			},
		},
		"integrations": bson.M{
			"integration1": bson.M{
				"role":      "writer",
				"invitedby": "user1",
				"disabled":  false,
			},
		},
		"personal": false,
	}

	_, err := col.InsertOne(ctx, testWorkspace)
	assert.NoError(t, err)

	// Create DBClient for migration
	c := mongox.NewClientWithDatabase(db)

	// Run migration
	err = AddWorkspaceMembersHash(ctx, c)
	assert.NoError(t, err)

	// Verify members_hash was added
	var result bson.M
	err = col.FindOne(ctx, bson.M{"_id": "workspace1"}).Decode(&result)
	assert.NoError(t, err)
	assert.Contains(t, result, "members_hash")
	assert.NotEmpty(t, result["members_hash"])

	// Verify hash is deterministic (run migration again and check same hash)
	originalHash := result["members_hash"]
	err = AddWorkspaceMembersHash(ctx, c)
	assert.NoError(t, err)

	err = col.FindOne(ctx, bson.M{"_id": "workspace1"}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, originalHash, result["members_hash"])
}

func TestReplaceWorkspaceAliasMembersIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongotest.Connect(t)(t)
	c := mongox.NewClientWithDatabase(db)

	// Run migration
	err := ReplaceWorkspaceAliasMembersIndex(ctx, c)
	assert.NoError(t, err)

	// Verify new index exists
	col := db.Collection("workspace")
	cursor, err := col.Indexes().List(ctx)
	assert.NoError(t, err)
	defer cursor.Close(ctx)

	var indexes []bson.M
	err = cursor.All(ctx, &indexes)
	assert.NoError(t, err)

	// Check for the new index
	found := false
	for _, index := range indexes {
		if name, ok := index["name"].(string); ok && name == "alias_members_hash_case_insensitive_unique" {
			found = true
			break
		}
	}
	assert.True(t, found, "New compound index alias_members_hash_case_insensitive_unique should exist")
}

func TestComputeMembersHashFromBSON(t *testing.T) {
	members := map[string]workspaceMemberDoc{
		"user1": {Role: "owner", InvitedBy: "user1", Disabled: false},
		"user2": {Role: "reader", InvitedBy: "user1", Disabled: true},
	}

	integrations := map[string]workspaceMemberDoc{
		"integration1": {Role: "writer", InvitedBy: "user1", Disabled: false},
	}

	hash1 := computeMembersHashFromBSON(members, integrations)
	hash2 := computeMembersHashFromBSON(members, integrations)

	// Hash should be deterministic
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA256 hex string length

	// Different members should produce different hash
	differentMembers := map[string]workspaceMemberDoc{
		"user3": {Role: "owner", InvitedBy: "user3", Disabled: false},
	}

	hash3 := computeMembersHashFromBSON(differentMembers, integrations)
	assert.NotEqual(t, hash1, hash3)
}
