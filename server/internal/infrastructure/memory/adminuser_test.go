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
