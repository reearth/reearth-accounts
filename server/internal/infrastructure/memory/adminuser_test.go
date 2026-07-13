package memory

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
)

func TestAdminUser_SaveAndFind(t *testing.T) {
	ctx := context.Background()
	r := NewAdminUser()

	u := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	assert.NoError(t, r.Save(ctx, u))

	got, err := r.FindByID(ctx, u.ID())
	assert.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())

	got, err = r.FindByEmail(ctx, "ALICE@eukarya.io")
	assert.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())

	_, err = r.FindByID(ctx, adminuser.NewID())
	assert.Equal(t, rerror.ErrNotFound, err)

	_, err = r.FindByEmail(ctx, "nobody@eukarya.io")
	assert.Equal(t, rerror.ErrNotFound, err)
}

func TestAdminUser_Save_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	r := NewAdminUser()

	u1 := adminuser.New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
	assert.NoError(t, r.Save(ctx, u1))

	// different id, same email -> rejected
	u2 := adminuser.New().NewID().Name("Alice2").Email("alice@eukarya.io").MustBuild()
	assert.Equal(t, adminuser.ErrDuplicatedAdminUser, r.Save(ctx, u2))

	// updating the same record keeps working
	u1.Approve(adminuser.NewID())
	assert.NoError(t, r.Save(ctx, u1))
}

func TestAdminUser_FindByIDs(t *testing.T) {
	ctx := context.Background()
	u1 := adminuser.New().NewID().Name("A").Email("a@eukarya.io").MustBuild()
	u2 := adminuser.New().NewID().Name("B").Email("b@eukarya.io").MustBuild()
	r := NewAdminUserWith(u1, u2)

	got, err := r.FindByIDs(ctx, adminuser.IDList{u1.ID(), u2.ID()})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))

	got, err = r.FindByIDs(ctx, adminuser.IDList{adminuser.NewID()})
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestAdminUser_List(t *testing.T) {
	ctx := context.Background()
	pending := adminuser.New().NewID().Name("P").Email("p@eukarya.io").MustBuild()
	approved := adminuser.New().NewID().Name("Q").Email("q@eukarya.io").Status(adminuser.StatusApproved).MustBuild()
	r := NewAdminUserWith(pending, approved)

	// no filter
	got, pi, err := r.List(ctx, adminuser.ListFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)

	// status filter
	st := adminuser.StatusPending
	got, pi, err = r.List(ctx, adminuser.ListFilter{Status: &st})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, pending.ID(), got[0].ID())
	assert.Equal(t, int64(1), pi.TotalCount)
}

func TestAdminUser_List_RejectsCursorPagination(t *testing.T) {
	ctx := context.Background()
	r := NewAdminUserWith(adminuser.New().NewID().Name("A").Email("a@eukarya.io").MustBuild())

	cur := usecasex.Cursor("x")
	first := int64(1)
	p := usecasex.CursorPagination{First: &first, After: &cur}.Wrap()
	_, _, err := r.List(ctx, adminuser.ListFilter{Pagination: p})
	assert.ErrorIs(t, err, adminuser.ErrCursorPaginationUnsupported)
}

func TestAdminUser_ExistsApprovedSystemAdminExcept(t *testing.T) {
	ctx := context.Background()

	sysAdmin := adminuser.New().NewID().Name("sys").Email("sys@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleSystemAdmin).MustBuild()
	viewer := adminuser.New().NewID().Name("viewer").Email("viewer@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleViewer).MustBuild()
	pendingSys := adminuser.New().NewID().Name("pending").Email("pending@eukarya.io").
		Status(adminuser.StatusPending).Role(adminuser.RoleSystemAdmin).MustBuild()

	r := NewAdminUserWith(sysAdmin, viewer, pendingSys)

	// excluding the only approved system_admin -> none left
	got, err := r.ExistsApprovedSystemAdminExcept(ctx, sysAdmin.ID())
	assert.NoError(t, err)
	assert.False(t, got)

	// excluding a viewer -> the approved system_admin still counts
	got, err = r.ExistsApprovedSystemAdminExcept(ctx, viewer.ID())
	assert.NoError(t, err)
	assert.True(t, got)

	// a pending system_admin is not counted, so excluding sysAdmin still yields none
	got, err = r.ExistsApprovedSystemAdminExcept(ctx, sysAdmin.ID())
	assert.NoError(t, err)
	assert.False(t, got)
}

func TestAdminUser_List_Pagination(t *testing.T) {
	ctx := context.Background()
	u1 := adminuser.New().NewID().Name("1").Email("1@eukarya.io").MustBuild()
	u2 := adminuser.New().NewID().Name("2").Email("2@eukarya.io").MustBuild()
	u3 := adminuser.New().NewID().Name("3").Email("3@eukarya.io").MustBuild()
	r := NewAdminUserWith(u1, u2, u3)

	p := usecasex.OffsetPagination{Offset: 1, Limit: 1}.Wrap()
	got, pi, err := r.List(ctx, adminuser.ListFilter{Pagination: p})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, int64(3), pi.TotalCount)
	assert.True(t, pi.HasNextPage)
	assert.True(t, pi.HasPreviousPage)
}
