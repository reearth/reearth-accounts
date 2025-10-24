package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestRole_FindAll(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	r := NewRole(mongox.NewClientWithDatabase(c))

	got, err := r.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, role.List{}, got)

	_, _ = c.Collection("role").InsertMany(ctx, []any{
		bson.M{"id": rid.String(), "name": "hoge"},
		bson.M{"id": rid2.String(), "name": "foo"},
	})

	got, err = r.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []role.ID{rid, rid2}, []role.ID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []string{"hoge", "foo"}, []string{got[0].Name(), got[1].Name()})
}

func TestRole_FindByID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("role").InsertMany(ctx, []any{
		bson.M{"id": rid.String(), "name": "hoge"},
		bson.M{"id": rid2.String(), "name": "foo"},
	})

	r := NewRole(mongox.NewClientWithDatabase(c))

	got, err := r.FindByID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, rid, got.ID())

	got, err = r.FindByID(ctx, id.NewRoleID())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestRole_FindByIDs(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("role").InsertMany(ctx, []any{
		bson.M{"id": rid.String(), "name": "hoge"},
		bson.M{"id": rid2.String(), "name": "foo"},
	})

	r := NewRole(mongox.NewClientWithDatabase(c))

	got, err := r.FindByIDs(ctx, []id.RoleID{rid, rid2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []id.RoleID{rid, rid2}, []id.RoleID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []string{"hoge", "foo"}, []string{got[0].Name(), got[1].Name()})

	got, err = r.FindByIDs(ctx, []id.RoleID{id.NewRoleID()})
	assert.NoError(t, err)
	assert.Equal(t, role.List{}, got)
}

func TestRole_Save(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	rid := id.NewRoleID()

	_, _ = c.Collection("role").InsertOne(ctx, bson.M{"id": rid.String(), "name": "hoge"})

	r := NewRole(mongox.NewClientWithDatabase(c))

	got, err := r.FindByID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, rid, got.ID())
	assert.Equal(t, "hoge", got.Name())

	got.Rename("foo")
	err = r.Save(ctx, *got)
	assert.NoError(t, err)

	got, err = r.FindByID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, rid, got.ID())
	assert.Equal(t, "foo", got.Name())
}

func TestRole_Remove(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	rid := id.NewRoleID()

	_, _ = c.Collection("role").InsertOne(ctx, bson.M{"id": rid.String(), "name": "hoge"})

	r := NewRole(mongox.NewClientWithDatabase(c))

	got, err := r.FindByID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, rid, got.ID())
	assert.Equal(t, "hoge", got.Name())

	err = r.Remove(ctx, rid)
	assert.NoError(t, err)

	got, err = r.FindByID(ctx, rid)
	assert.Error(t, err)
	assert.Nil(t, got)
}
