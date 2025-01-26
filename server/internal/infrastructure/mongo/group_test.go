package mongo

import (
	"context"
	"testing"

	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestGroup_FindAll(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	gid := id.NewGroupID()
	gid2 := id.NewGroupID()

	g := NewGroup(mongox.NewClientWithDatabase(c))

	got, err := g.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, group.List{}, got)

	_, _ = c.Collection("group").InsertMany(ctx, []any{
		bson.M{"id": gid.String(), "name": "hoge"},
		bson.M{"id": gid2.String(), "name": "foo"},
	})

	got, err = g.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []group.ID{gid, gid2}, []group.ID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []string{"hoge", "foo"}, []string{got[0].Name(), got[1].Name()})
}

func TestGroup_FindByID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	gid := id.NewGroupID()
	gid2 := id.NewGroupID()

	_, _ = c.Collection("group").InsertMany(ctx, []any{
		bson.M{"id": gid.String(), "name": "hoge"},
		bson.M{"id": gid2.String(), "name": "foo"},
	})

	g := NewGroup(mongox.NewClientWithDatabase(c))

	got, err := g.FindByID(ctx, gid)
	assert.NoError(t, err)
	assert.Equal(t, gid, got.ID())

	got, err = g.FindByID(ctx, id.NewGroupID())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGroup_FindByIDs(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	gid := id.NewGroupID()
	gid2 := id.NewGroupID()

	_, _ = c.Collection("group").InsertMany(ctx, []any{
		bson.M{"id": gid.String(), "name": "hoge"},
		bson.M{"id": gid2.String(), "name": "foo"},
	})

	g := NewGroup(mongox.NewClientWithDatabase(c))

	got, err := g.FindByIDs(ctx, []id.GroupID{gid, gid2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []id.GroupID{gid, gid2}, []id.GroupID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []string{"hoge", "foo"}, []string{got[0].Name(), got[1].Name()})

	got, err = g.FindByIDs(ctx, []id.GroupID{id.NewGroupID()})
	assert.NoError(t, err)
	assert.Equal(t, group.List{}, got)
}

func TestGroup_Save(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	gid := id.NewGroupID()

	_, _ = c.Collection("group").InsertOne(ctx, bson.M{"id": gid.String(), "name": "hoge"})

	g := NewGroup(mongox.NewClientWithDatabase(c))

	got, err := g.FindByID(ctx, gid)
	assert.NoError(t, err)
	assert.Equal(t, gid, got.ID())
	assert.Equal(t, "hoge", got.Name())

	got.Rename("foo")
	err = g.Save(ctx, *got)
	assert.NoError(t, err)

	got, err = g.FindByID(ctx, gid)
	assert.NoError(t, err)
	assert.Equal(t, gid, got.ID())
	assert.Equal(t, "foo", got.Name())
}

func TestGroup_Remove(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	gid := id.NewGroupID()

	_, _ = c.Collection("group").InsertOne(ctx, bson.M{"id": gid.String(), "name": "hoge"})

	g := NewGroup(mongox.NewClientWithDatabase(c))

	got, err := g.FindByID(ctx, gid)
	assert.NoError(t, err)
	assert.Equal(t, gid, got.ID())
	assert.Equal(t, "hoge", got.Name())

	err = g.Remove(ctx, gid)
	assert.NoError(t, err)

	got, err = g.FindByID(ctx, gid)
	assert.Error(t, err)
	assert.Nil(t, got)
}
