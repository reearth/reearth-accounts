package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestPermittable_FindByUserID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	pid := id.NewPermittableID()
	pid2 := id.NewPermittableID()
	uid := user.NewID()
	uid2 := user.NewID()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("permittable").InsertMany(ctx, []any{
		bson.M{"id": pid.String(), "userid": uid.String(), "roleids": []string{rid.String()}},
		bson.M{"id": pid2.String(), "userid": uid2.String(), "roleids": []string{rid2.String()}},
	})

	p := NewPermittable(mongox.NewClientWithDatabase(c))

	got, err := p.FindByUserID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, pid, got.ID())
	assert.Equal(t, uid, got.UserID())
	assert.Equal(t, []id.RoleID{rid}, got.RoleIDs())

	got, err = p.FindByUserID(ctx, user.NewID())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestPermittable_FindByUserIDs(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	pid := id.NewPermittableID()
	pid2 := id.NewPermittableID()
	uid := user.NewID()
	uid2 := user.NewID()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("permittable").InsertMany(ctx, []any{
		bson.M{"id": pid.String(), "userid": uid.String(), "roleids": []string{rid.String()}},
		bson.M{"id": pid2.String(), "userid": uid2.String(), "roleids": []string{rid2.String()}},
	})

	p := NewPermittable(mongox.NewClientWithDatabase(c))

	got, err := p.FindByUserIDs(ctx, []user.ID{uid, uid2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []id.PermittableID{pid, pid2}, []id.PermittableID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []user.ID{uid, uid2}, []user.ID{got[0].UserID(), got[1].UserID()})
	assert.ElementsMatch(t, [][]id.RoleID{{rid}, {rid2}}, [][]id.RoleID{got[0].RoleIDs(), got[1].RoleIDs()})

	got, err = p.FindByUserIDs(ctx, []user.ID{user.NewID()})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestPermittable_FindByRoleID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	pid := id.NewPermittableID()
	pid2 := id.NewPermittableID()
	pid3 := id.NewPermittableID()
	uid := user.NewID()
	uid2 := user.NewID()
	uid3 := user.NewID()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("permittable").InsertMany(ctx, []any{
		bson.M{"id": pid.String(), "userid": uid.String(), "roleids": []string{rid.String()}},
		bson.M{"id": pid2.String(), "userid": uid2.String(), "roleids": []string{rid2.String()}},
		bson.M{"id": pid3.String(), "userid": uid3.String(), "roleids": []string{rid.String(), rid2.String()}},
	})

	p := NewPermittable(mongox.NewClientWithDatabase(c))

	got, err := p.FindByRoleID(ctx, rid)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.ElementsMatch(t, []id.PermittableID{pid, pid3}, []id.PermittableID{got[0].ID(), got[1].ID()})
	assert.ElementsMatch(t, []user.ID{uid, uid3}, []user.ID{got[0].UserID(), got[1].UserID()})
	assert.ElementsMatch(t, [][]id.RoleID{{rid}, {rid, rid2}}, [][]id.RoleID{got[0].RoleIDs(), got[1].RoleIDs()})

	got, err = p.FindByUserIDs(ctx, []user.ID{user.NewID()})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestPermittable_Save(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	pid := id.NewPermittableID()
	uid := user.NewID()
	rid := id.NewRoleID()
	rid2 := id.NewRoleID()

	_, _ = c.Collection("permittable").InsertOne(ctx, bson.M{"id": pid.String(), "userid": uid.String(), "roleids": []string{rid.String()}})

	p := NewPermittable(mongox.NewClientWithDatabase(c))

	got, err := p.FindByUserID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, pid, got.ID())
	assert.Equal(t, uid, got.UserID())
	assert.Equal(t, []id.RoleID{rid}, got.RoleIDs())

	got.EditRoleIDs([]id.RoleID{rid2})
	err = p.Save(ctx, *got)
	assert.NoError(t, err)

	got, err = p.FindByUserID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, pid, got.ID())
	assert.Equal(t, uid, got.UserID())
	assert.Equal(t, []id.RoleID{rid2}, got.RoleIDs())
}
