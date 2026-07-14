//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminUser_SaveAndFind(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	u := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	require.NoError(t, r.Save(ctx, u))

	got, err := r.FindByID(ctx, u.ID())
	require.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())
	assert.Equal(t, adminuser.StatusPending, got.Status())

	got, err = r.FindByEmail(ctx, "ALICE@eukarya.io")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())

	_, err = r.FindByID(ctx, adminuser.NewID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	_, err = r.FindByEmail(ctx, "nobody@eukarya.io")
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func TestAdminUser_Save_Approve(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	u := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	require.NoError(t, r.Save(ctx, u))

	approver := adminuser.NewID()
	u.Approve(approver)
	require.NoError(t, r.Save(ctx, u))

	got, err := r.FindByID(ctx, u.ID())
	require.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.Equal(t, approver, got.ApprovedBy())
	assert.False(t, got.ApprovedAt().IsZero())
}

func TestAdminUser_Save_DuplicateEmail(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	u1 := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	require.NoError(t, r.Save(ctx, u1))

	// different id, case-different email -> duplicate
	u2 := adminuser.New().NewID().Name("Alice2").Email("ALICE@eukarya.io").MustBuild()
	assert.ErrorIs(t, r.Save(ctx, u2), adminuser.ErrDuplicatedAdminUser)
}

func TestAdminUser_FindByIDs(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	u1 := adminuser.New().NewID().Name("A").Email("a@eukarya.io").MustBuild()
	u2 := adminuser.New().NewID().Name("B").Email("b@eukarya.io").MustBuild()
	require.NoError(t, r.Save(ctx, u1))
	require.NoError(t, r.Save(ctx, u2))

	// requested-order is preserved (u2 before u1)
	got, err := r.FindByIDs(ctx, adminuser.IDList{u2.ID(), u1.ID()})
	require.NoError(t, err)
	require.Equal(t, 2, len(got))
	assert.Equal(t, u2.ID(), got[0].ID())
	assert.Equal(t, u1.ID(), got[1].ID())

	got, err = r.FindByIDs(ctx, adminuser.IDList{adminuser.NewID()})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestAdminUser_List(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	p1 := adminuser.New().NewID().Name("P1").Email("p1@eukarya.io").MustBuild()
	p2 := adminuser.New().NewID().Name("P2").Email("p2@eukarya.io").MustBuild()
	a1 := adminuser.New().NewID().Name("A1").Email("a1@eukarya.io").Status(adminuser.StatusApproved).MustBuild()
	for _, u := range []*adminuser.AdminUser{p1, p2, a1} {
		require.NoError(t, r.Save(ctx, u))
	}

	// no filter
	got, pi, err := r.List(ctx, adminuser.ListFilter{Pagination: usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap()})
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
	assert.Equal(t, int64(3), pi.TotalCount)

	// status filter
	st := adminuser.StatusPending
	got, pi, err = r.List(ctx, adminuser.ListFilter{Status: &st, Pagination: usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap()})
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)

	// pagination
	got, pi, err = r.List(ctx, adminuser.ListFilter{Pagination: usecasex.OffsetPagination{Offset: 1, Limit: 1}.Wrap()})
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, int64(3), pi.TotalCount)
	assert.True(t, pi.HasNextPage)
	assert.True(t, pi.HasPreviousPage)
}

func TestAdminUser_ExistsApprovedSystemAdminExcept(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	sysAdmin := adminuser.New().NewID().Name("sys").Email("sys@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleSystemAdmin).MustBuild()
	viewer := adminuser.New().NewID().Name("viewer").Email("viewer@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleViewer).MustBuild()
	pendingSys := adminuser.New().NewID().Name("pending").Email("pending@eukarya.io").
		Status(adminuser.StatusPending).Role(adminuser.RoleSystemAdmin).MustBuild()
	require.NoError(t, r.Save(ctx, sysAdmin))
	require.NoError(t, r.Save(ctx, viewer))
	require.NoError(t, r.Save(ctx, pendingSys))

	// excluding the only approved system_admin -> none left
	got, err := r.ExistsApprovedSystemAdminExcept(ctx, sysAdmin.ID())
	require.NoError(t, err)
	assert.False(t, got)

	// excluding a viewer -> the approved system_admin still counts
	got, err = r.ExistsApprovedSystemAdminExcept(ctx, viewer.ID())
	require.NoError(t, err)
	assert.True(t, got)
}

func TestAdminUser_List_RoleFilter(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewAdminUser(NewClient(pool))

	admin := adminuser.New().NewID().Name("A").Email("a@eukarya.io").Role(adminuser.RoleSystemAdmin).Status(adminuser.StatusApproved).MustBuild()
	viewer := adminuser.New().NewID().Name("V").Email("v@eukarya.io").Role(adminuser.RoleViewer).Status(adminuser.StatusApproved).MustBuild()
	pendingViewer := adminuser.New().NewID().Name("PV").Email("pv@eukarya.io").Role(adminuser.RoleViewer).Status(adminuser.StatusPending).MustBuild()
	for _, u := range []*adminuser.AdminUser{admin, viewer, pendingViewer} {
		require.NoError(t, r.Save(ctx, u))
	}

	p := usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap()

	// role filter: system_admin
	sysAdmin := adminuser.RoleSystemAdmin
	got, pi, err := r.List(ctx, adminuser.ListFilter{Role: &sysAdmin, Pagination: p})
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, admin.ID(), got[0].ID())
	assert.Equal(t, int64(1), pi.TotalCount)

	// role filter: viewer
	viewerRole := adminuser.RoleViewer
	got, pi, err = r.List(ctx, adminuser.ListFilter{Role: &viewerRole, Pagination: p})
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)

	// combined status + role
	approved := adminuser.StatusApproved
	got, pi, err = r.List(ctx, adminuser.ListFilter{Status: &approved, Role: &viewerRole, Pagination: p})
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, viewer.ID(), got[0].ID())
	assert.Equal(t, int64(1), pi.TotalCount)
}
