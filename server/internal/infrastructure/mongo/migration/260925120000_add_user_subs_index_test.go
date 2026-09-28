package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestUserSubsIndexModels(t *testing.T) {
	models := userSubsIndexModels()
	require.Len(t, models, 3)

	tests := []struct {
		key    string
		name   string
		sparse bool
	}{
		{key: "subs", name: "subs", sparse: false},
		{key: "auth0sub", name: "auth0sub_sparse", sparse: true},
		{key: "auth0sublist", name: "auth0sublist_sparse", sparse: true},
	}
	for i, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := models[i]
			assert.Equal(t, bson.D{{Key: tt.key, Value: 1}}, m.Keys)
			require.NotNil(t, m.Options)
			require.NotNil(t, m.Options.Name)
			assert.Equal(t, tt.name, *m.Options.Name)
			if tt.sparse {
				require.NotNil(t, m.Options.Sparse)
				assert.True(t, *m.Options.Sparse)
			} else {
				assert.True(t, m.Options.Sparse == nil || !*m.Options.Sparse)
			}
			assert.Nil(t, m.Options.Unique, "sub indexes must not be unique")
		})
	}
}

func findBySubFilter(sub string) bson.M {
	return bson.M{
		"$or": []bson.M{
			{"subs": bson.M{"$elemMatch": bson.M{"$eq": sub}}},
			{"auth0sub": sub},
			{"auth0sublist": bson.M{"$elemMatch": bson.M{"$eq": sub}}},
		},
	}
}

func containsStage(v any, stage string) bool {
	switch x := v.(type) {
	case bson.M:
		if s, ok := x["stage"].(string); ok && s == stage {
			return true
		}
		for _, vv := range x {
			if containsStage(vv, stage) {
				return true
			}
		}
	case bson.D:
		for _, e := range x {
			if e.Key == "stage" {
				if s, ok := e.Value.(string); ok && s == stage {
					return true
				}
			}
			if containsStage(e.Value, stage) {
				return true
			}
		}
	case bson.A:
		for _, vv := range x {
			if containsStage(vv, stage) {
				return true
			}
		}
	}
	return false
}

func TestAddUserSubsIndexes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongotest.Connect(t)(t)
	col := db.Collection("user")

	users := []bson.M{
		{"_id": primitive.NewObjectID(), "id": "u-subs", "name": "subs", "subs": bson.A{"auth0|subs-1", "google|subs-2"}},
		{"_id": primitive.NewObjectID(), "id": "u-auth0sub", "name": "legacy", "auth0sub": "auth0|legacy"},
		{"_id": primitive.NewObjectID(), "id": "u-auth0sublist", "name": "legacylist", "auth0sublist": bson.A{"auth0|list-1", "auth0|list-2"}},
	}
	for _, u := range users {
		_, err := col.InsertOne(ctx, u)
		require.NoError(t, err)
	}

	c := mongox.NewClientWithDatabase(db)
	require.NoError(t, AddUserSubsIndexes(ctx, c))

	cursor, err := col.Indexes().List(ctx)
	require.NoError(t, err)
	var indexes []bson.M
	require.NoError(t, cursor.All(ctx, &indexes))

	byName := map[string]bson.M{}
	for _, idx := range indexes {
		if name, ok := idx["name"].(string); ok {
			byName[name] = idx
		}
	}
	for _, name := range []string{"subs", "auth0sub_sparse", "auth0sublist_sparse"} {
		assert.Contains(t, byName, name)
	}
	assert.Nil(t, byName["subs"]["unique"])
	assert.Equal(t, true, byName["auth0sub_sparse"]["sparse"])
	assert.Equal(t, true, byName["auth0sublist_sparse"]["sparse"])

	require.NoError(t, AddUserSubsIndexes(ctx, c))

	for sub, wantID := range map[string]string{
		"auth0|subs-1":  "u-subs",
		"google|subs-2": "u-subs",
		"auth0|legacy":  "u-auth0sub",
		"auth0|list-2":  "u-auth0sublist",
	} {
		var result bson.M
		require.NoError(t, col.FindOne(ctx, findBySubFilter(sub)).Decode(&result), sub)
		assert.Equal(t, wantID, result["id"], sub)
	}
	var result bson.M
	assert.ErrorIs(t, col.FindOne(ctx, findBySubFilter("nonexistent")).Decode(&result), mongo.ErrNoDocuments)

	var explain bson.M
	err = db.RunCommand(ctx, bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: "user"},
			{Key: "filter", Value: findBySubFilter("auth0|subs-1")},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}).Decode(&explain)
	require.NoError(t, err)
	winning := explain["queryPlanner"].(bson.M)["winningPlan"]
	assert.False(t, containsStage(winning, "COLLSCAN"), "FindBySub should not use COLLSCAN: %v", winning)
	assert.True(t, containsStage(winning, "IXSCAN"), "FindBySub should use IXSCAN: %v", winning)
}
