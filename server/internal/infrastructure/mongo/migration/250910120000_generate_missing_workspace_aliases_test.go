package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestGenerateMissingWorkspaceAliases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	workspaceCollection := db.Collection("workspace")
	
	testWorkspaces := []interface{}{
		bson.M{
			"_id":   "workspace1",
			"id":    "workspace1",
			"name":  "Workspace One",
			"alias": "", 
		},
		bson.M{
			"_id":   "workspace2", 
			"id":    "workspace2",
			"name":  "Test Workspace",
			"alias": "test", 
		},
		bson.M{
			"_id":   "workspace3",
			"id":    "workspace3", 
			"name":  "Another Workspace",
			"alias": "aaaaa", 
		},
		bson.M{
			"_id":   "workspace4",
			"id":    "workspace4",
			"name":  "E2E Workspace", 
			"alias": "e2e-workspace-name", 
		},
		bson.M{
			"_id":   "workspace5",
			"id":    "workspace5",
			"name":  "Valid Workspace",
			"alias": "validalias", 
		},
		bson.M{
			"_id":   "01jhmkh59s3q06xzm1215w7y2v",
			"id":    "01jhmkh59s3q06xzm1215w7y2v",
			"name":  "Eukarya Workspace",
			"alias": "eukarya", 
		},
	}

	_, err := workspaceCollection.InsertMany(ctx, testWorkspaces)
	assert.NoError(t, err)

	// Run the migration
	err = GenerateMissingWorkspaceAliases(ctx, mongoxClient)
	assert.NoError(t, err)

	// Check that problematic workspaces got new random aliases
	problematicWorkspaces := map[string]string{
		"workspace1": "", // originally empty
		"workspace2": "test", // originally "test"
		"workspace3": "aaaaa", // originally "aaaaa"
		"workspace4": "e2e-workspace-name", // originally "e2e-workspace-name"
	}
	
	for id, originalAlias := range problematicWorkspaces {
		var result bson.M
		err := workspaceCollection.FindOne(ctx, bson.M{"id": id}).Decode(&result)
		assert.NoError(t, err)
		
		alias, exists := result["alias"]
		assert.True(t, exists, "Alias should exist for workspace %s", id)
		aliasStr := alias.(string)
		assert.NotEmpty(t, aliasStr, "Alias should not be empty for workspace %s", id)
		assert.Len(t, aliasStr, 10, "Generated alias should be 10 characters for workspace %s", id)
		assert.NotEqual(t, originalAlias, aliasStr, "Original problematic alias should be replaced for workspace %s", id)
	}

	// Check that workspace5 kept its original alias
	var result bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "workspace5"}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "validalias", result["alias"], "Valid alias should not be changed")

	// Check that the specific eukarya workspace got updated to eukarya-roboco
	var eukaryaResult bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "01jhmkh59s3q06xzm1215w7y2v"}).Decode(&eukaryaResult)
	assert.NoError(t, err)
	assert.Equal(t, "eukarya-roboco", eukaryaResult["alias"], "Eukarya workspace should be updated to eukarya-roboco")
}