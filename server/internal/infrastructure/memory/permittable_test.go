package memory

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/stretchr/testify/assert"
)

func TestNewPermittable(t *testing.T) {
	repo := NewPermittable()
	assert.NotNil(t, repo)
}

func TestNewPermittableWith(t *testing.T) {
	uid := user.NewID()
	rid := id.NewRoleID()
	p, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid}).
		Build()

	repo := NewPermittableWith(p)
	assert.NotNil(t, repo)
}

func TestPermittable_FindByUserID(t *testing.T) {
	uid := user.NewID()
	uid2 := user.NewID()
	rid := id.NewRoleID()
	p1, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid}).
		Build()
	p2, _ := permittable.New().
		NewID().
		UserID(uid2).
		RoleIDs([]id.RoleID{rid}).
		Build()
	repo := NewPermittableWith(p1, p2)

	ctx := context.Background()
	out, err := repo.FindByUserID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, p1, out)

	out, err = repo.FindByUserID(ctx, user.NewID())
	assert.Error(t, err)
	assert.Nil(t, out)
}

func TestPermittable_FindByUserIDs(t *testing.T) {
	uid := user.NewID()
	uid2 := user.NewID()
	uid3 := user.NewID()
	rid := id.NewRoleID()
	p1, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid}).
		Build()
	p2, _ := permittable.New().
		NewID().
		UserID(uid2).
		RoleIDs([]id.RoleID{rid}).
		Build()
	p3, _ := permittable.New().
		NewID().
		UserID(uid3).
		RoleIDs([]id.RoleID{rid}).
		Build()
	repo := NewPermittableWith(p1, p2, p3)

	ctx := context.Background()
	out, err := repo.FindByUserIDs(ctx, user.IDList{uid, uid2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(out))
	assert.ElementsMatch(t, permittable.List{p1, p2}, out)

	out, err = repo.FindByUserIDs(ctx, user.IDList{user.NewID(), user.NewID()})
	assert.Error(t, err)
	assert.Equal(t, 0, len(out))
}

func TestPermittable_FindByRoleID(t *testing.T) {
	uid := user.NewID()
	uid2 := user.NewID()
	uid3 := user.NewID()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()
	rid3 := id.NewRoleID()
	p1, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid, rid2}).
		Build()
	p2, _ := permittable.New().
		NewID().
		UserID(uid2).
		RoleIDs([]id.RoleID{rid2, rid3}).
		Build()
	p3, _ := permittable.New().
		NewID().
		UserID(uid3).
		RoleIDs([]id.RoleID{rid3, rid}).
		Build()
	repo := NewPermittableWith(p1, p2, p3)

	ctx := context.Background()
	out, err := repo.FindByRoleID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(out))
	assert.ElementsMatch(t, permittable.List{p1, p3}, out)

	out, err = repo.FindByRoleID(ctx, id.NewRoleID())
	assert.Error(t, err)
	assert.Equal(t, 0, len(out))
}

func TestPermittable_Save(t *testing.T) {
	uid := user.NewID()
	rid := id.NewRoleID()
	p1, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid}).
		Build()
	repo := NewPermittable()

	ctx := context.Background()
	err := repo.Save(ctx, *p1)
	assert.NoError(t, err)

	out, err := repo.FindByUserID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, p1, out)
}
