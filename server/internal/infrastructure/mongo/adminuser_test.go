package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAdminUser_FindByIDAndEmail(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	uid := adminuser.NewID()
	now := time.Now()

	_, _ = c.Collection("adminuser").InsertOne(ctx, bson.M{
		"id":        uid.String(),
		"email":     "alice@eukarya.io",
		"name":      "Alice",
		"status":    "pending",
		"createdat": now,
		"updatedat": now,
	})

	r := NewAdminUser(mongox.NewClientWithDatabase(c))

	got, err := r.FindByID(ctx, uid)
	assert.NoError(t, err)
	assert.Equal(t, uid, got.ID())
	assert.Equal(t, "alice@eukarya.io", got.Email())

	got, err = r.FindByEmail(ctx, "ALICE@eukarya.io")
	assert.NoError(t, err)
	assert.Equal(t, uid, got.ID())

	got, err = r.FindByID(ctx, adminuser.NewID())
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestAdminUser_FindByIDs(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	uid := adminuser.NewID()
	uid2 := adminuser.NewID()
	now := time.Now()

	_, _ = c.Collection("adminuser").InsertMany(ctx, []any{
		bson.M{"id": uid.String(), "email": "a@eukarya.io", "name": "A", "status": "pending", "createdat": now, "updatedat": now},
		bson.M{"id": uid2.String(), "email": "b@eukarya.io", "name": "B", "status": "approved", "createdat": now, "updatedat": now},
	})

	r := NewAdminUser(mongox.NewClientWithDatabase(c))

	got, err := r.FindByIDs(ctx, adminuser.IDList{uid, uid2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, uid, got[0].ID()) // order preserved
	assert.Equal(t, uid2, got[1].ID())

	got, err = r.FindByIDs(ctx, adminuser.IDList{adminuser.NewID()})
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestAdminUser_Save(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()

	r := NewAdminUser(mongox.NewClientWithDatabase(c))

	u := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	assert.NoError(t, r.Save(ctx, u))

	got, err := r.FindByID(ctx, u.ID())
	assert.NoError(t, err)
	assert.Equal(t, adminuser.StatusPending, got.Status())

	got.Approve(adminuser.NewID())
	assert.NoError(t, r.Save(ctx, got))

	got, err = r.FindByID(ctx, u.ID())
	assert.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.False(t, got.ApprovedBy().IsEmpty())
}

func TestAdminUser_List(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	now := time.Now()

	_, _ = c.Collection("adminuser").InsertMany(ctx, []any{
		bson.M{"id": adminuser.NewID().String(), "email": "p1@eukarya.io", "name": "P1", "status": "pending", "createdat": now.Add(-2 * time.Hour), "updatedat": now},
		bson.M{"id": adminuser.NewID().String(), "email": "p2@eukarya.io", "name": "P2", "status": "pending", "createdat": now.Add(-1 * time.Hour), "updatedat": now},
		bson.M{"id": adminuser.NewID().String(), "email": "a1@eukarya.io", "name": "A1", "status": "approved", "createdat": now, "updatedat": now},
	})

	r := NewAdminUser(mongox.NewClientWithDatabase(c))

	// status filter + creation order
	st := adminuser.StatusPending
	got, pi, err := r.List(ctx, adminuser.ListFilter{Status: &st, Pagination: usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap()})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, "p1@eukarya.io", got[0].Email())
	assert.Equal(t, "p2@eukarya.io", got[1].Email())
	assert.Equal(t, int64(2), pi.TotalCount)
}

func TestAdminUser_ExistsApprovedSystemAdminExcept(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	r := NewAdminUser(mongox.NewClientWithDatabase(c))

	sysAdmin := adminuser.New().NewID().Name("sys").Email("sys@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleSystemAdmin).MustBuild()
	viewer := adminuser.New().NewID().Name("viewer").Email("viewer@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleViewer).MustBuild()
	pendingSys := adminuser.New().NewID().Name("pending").Email("pending@eukarya.io").
		Status(adminuser.StatusPending).Role(adminuser.RoleSystemAdmin).MustBuild()
	assert.NoError(t, r.Save(ctx, sysAdmin))
	assert.NoError(t, r.Save(ctx, viewer))
	assert.NoError(t, r.Save(ctx, pendingSys))

	// excluding the only approved system_admin -> none left
	got, err := r.ExistsApprovedSystemAdminExcept(ctx, sysAdmin.ID())
	assert.NoError(t, err)
	assert.False(t, got)

	// excluding a viewer -> the approved system_admin still counts
	got, err = r.ExistsApprovedSystemAdminExcept(ctx, viewer.ID())
	assert.NoError(t, err)
	assert.True(t, got)
}
