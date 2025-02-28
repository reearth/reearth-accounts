package memory

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/stretchr/testify/assert"
)

func TestNewRole(t *testing.T) {
	repo := NewRole()
	assert.NotNil(t, repo)
}

func TestNewRoleWith(t *testing.T) {
	rid := id.NewRoleID()
	r1, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()

	repo := NewRoleWith(r1)
	assert.NotNil(t, repo)
}

func TestRole_FindAll(t *testing.T) {
	ctx := context.Background()
	repo := NewRole()

	out, err := repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, role.List{}, out)

	r1, _ := role.New().
		NewID().
		Name("hoge").
		Build()
	r2, _ := role.New().
		NewID().
		Name("foo").
		Build()
	repo = NewRoleWith(r1, r2)

	out, err = repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.ElementsMatch(t, role.List{r1, r2}, out)
}

func TestRole_FindByID(t *testing.T) {
	rid := id.NewRoleID()
	r1, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()
	repo := NewRoleWith(r1)

	ctx := context.Background()
	out, err := repo.FindByID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, r1, out)

	got, err := repo.FindByID(ctx, id.RoleID{})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestRole_FindByIDs(t *testing.T) {
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()
	r1, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()
	r2, _ := role.New().
		ID(rid2).
		Name("foo").
		Build()
	repo := NewRoleWith(r1, r2)

	ctx := context.Background()
	out, err := repo.FindByIDs(ctx, id.RoleIDList{rid, rid2})
	assert.NoError(t, err)
	assert.ElementsMatch(t, role.List{r1, r2}, out)

	out, err = repo.FindByIDs(ctx, id.RoleIDList{id.NewRoleID()})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(out))
}

func TestRole_Save(t *testing.T) {
	r1, _ := role.New().
		NewID().
		Name("hoge").
		Build()
	repo := NewRole()

	ctx := context.Background()
	err := repo.Save(ctx, *r1)
	assert.NoError(t, err)

	out, err := repo.FindByID(ctx, r1.ID())
	assert.NoError(t, err)
	assert.Equal(t, r1, out)
}

func TestRole_Remove(t *testing.T) {
	r1, _ := role.New().
		NewID().
		Name("hoge").
		Build()
	repo := NewRoleWith(r1)

	ctx := context.Background()
	err := repo.Remove(ctx, r1.ID())
	assert.NoError(t, err)

	out, err := repo.FindByID(ctx, r1.ID())
	assert.Error(t, err)
	assert.Nil(t, out)
}
