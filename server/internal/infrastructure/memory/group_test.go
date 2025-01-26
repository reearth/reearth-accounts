package memory

import (
	"context"
	"testing"

	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/stretchr/testify/assert"
)

func TestNewGroup(t *testing.T) {
	repo := NewGroup()
	assert.NotNil(t, repo)
}

func TestNewGroupWith(t *testing.T) {
	gid := id.NewGroupID()
	g1, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()

	repo := NewGroupWith(g1)
	assert.NotNil(t, repo)
}

func TestGroup_FindAll(t *testing.T) {
	ctx := context.Background()
	repo := NewGroup()

	out, err := repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, group.List{}, out)

	g1, _ := group.New().
		NewID().
		Name("hoge").
		Build()
	g2, _ := group.New().
		NewID().
		Name("foo").
		Build()
	repo = NewGroupWith(g1, g2)

	out, err = repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.ElementsMatch(t, group.List{g1, g2}, out)
}

func TestGroup_FindByID(t *testing.T) {
	gid := id.NewGroupID()
	g1, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()
	repo := NewGroupWith(g1)

	ctx := context.Background()
	out, err := repo.FindByID(ctx, gid)
	assert.NoError(t, err)
	assert.Equal(t, g1, out)

	got, err := repo.FindByID(ctx, id.GroupID{})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGroup_FindByIDs(t *testing.T) {
	gid := id.NewGroupID()
	gid2 := id.NewGroupID()
	g1, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()
	g2, _ := group.New().
		ID(gid2).
		Name("foo").
		Build()
	repo := NewGroupWith(g1, g2)

	ctx := context.Background()
	out, err := repo.FindByIDs(ctx, id.GroupIDList{gid, gid2})
	assert.NoError(t, err)
	assert.ElementsMatch(t, group.List{g1, g2}, out)

	out, err = repo.FindByIDs(ctx, id.GroupIDList{id.NewGroupID()})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(out))
}

func TestGroup_Save(t *testing.T) {
	g1, _ := group.New().
		NewID().
		Name("hoge").
		Build()
	repo := NewGroup()

	ctx := context.Background()
	err := repo.Save(ctx, *g1)
	assert.NoError(t, err)

	out, err := repo.FindByID(ctx, g1.ID())
	assert.NoError(t, err)
	assert.Equal(t, g1, out)
}

func TestGroup_Remove(t *testing.T) {
	g1, _ := group.New().
		NewID().
		Name("hoge").
		Build()
	repo := NewGroupWith(g1)

	ctx := context.Background()
	err := repo.Remove(ctx, g1.ID())
	assert.NoError(t, err)

	out, err := repo.FindByID(ctx, g1.ID())
	assert.Error(t, err)
	assert.Nil(t, out)
}
