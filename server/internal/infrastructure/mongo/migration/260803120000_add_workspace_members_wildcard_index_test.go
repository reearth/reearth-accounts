package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestWorkspaceMembersWildcardIndexModel(t *testing.T) {
	model := workspaceMembersWildcardIndexModel()
	assert.Equal(t, bson.D{{Key: "members.$**", Value: 1}}, model.Keys)
}

func TestAddWorkspaceMembersWildcardIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongotest.Connect(t)(t)
	col := db.Collection("workspace")

	testWorkspace := bson.M{
		"_id":   primitive.NewObjectID(),
		"alias": "test-workspace",
		"name":  "Test Workspace",
		"email": "test@example.com",
		"members": bson.M{
			"user1": bson.M{"role": "owner", "invitedby": "user1", "disabled": false},
		},
		"personal": false,
	}
	_, err := col.InsertOne(ctx, testWorkspace)
	assert.NoError(t, err)

	c := mongox.NewClientWithDatabase(db)
	assert.NoError(t, AddWorkspaceMembersWildcardIndex(ctx, c))

	// index exists
	cursor, err := col.Indexes().List(ctx)
	assert.NoError(t, err)
	defer func() {
		_ = cursor.Close(ctx)
	}()
	var indexes []bson.M
	assert.NoError(t, cursor.All(ctx, &indexes))

	found := false
	for _, index := range indexes {
		if keys, ok := index["key"].(bson.M); ok {
			if _, ok := keys["members.$**"]; ok {
				found = true
			}
		}
	}
	assert.True(t, found, "wildcard index on workspace.members should exist")

	// the exact query used by FindByUser should still work correctly with the index in place
	var result bson.M
	err = col.FindOne(ctx, bson.M{"members.user1": bson.M{"$exists": true}}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, testWorkspace["alias"], result["alias"])

	err = col.FindOne(ctx, bson.M{"members.nonexistentuser": bson.M{"$exists": true}}).Decode(&result)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}
