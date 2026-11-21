package infrastructure

import (
	"context"
	"testing"

	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestNew_CreatesNoPolicyCollections(t *testing.T) {
	mongotest.Env = "REEARTH_DB"
	ctx := context.Background()
	db := mongotest.Connect(t)(t)

	_, err := New(ctx, db.Client(), db.Name(), false, false, nil)
	require.NoError(t, err)

	names, err := db.ListCollectionNames(ctx, bson.M{})
	require.NoError(t, err)
	assert.NotContains(t, names, "policy_types")
	assert.NotContains(t, names, "policy_versions")
}
