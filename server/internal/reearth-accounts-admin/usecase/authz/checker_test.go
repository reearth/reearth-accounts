package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/rbac"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// newTestClient returns a non-nil Cerbos client. cerbos.New is lazy and does
// not dial, so tests can drive the pre-Cerbos code paths (repo lookups) without
// a running Cerbos server.
func newTestClient(t *testing.T) *cerbos.GRPCClient {
	t.Helper()
	client, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A nil client (Cerbos unconfigured, e.g. local dev) must bypass authorization
// and never touch the repositories.
func TestChecker_Allowed_NilClientBypasses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// No EXPECT() calls: gomock fails the test if either repo is touched.
	mockRoleRepo := role.NewMockRepo(ctrl)
	mockPermittableRepo := permittable.NewMockRepo(ctrl)

	c := NewChecker(nil, mockRoleRepo, mockPermittableRepo)

	allowed, err := c.Allowed(context.Background(), user.NewID(), adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// When the caller has no permittable record, the request must be denied without
// consulting Cerbos. A not-found error from the repo is treated as "no roles".
func TestChecker_Allowed_NoPermittableDenies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uid := user.NewID()
	mockRoleRepo := role.NewMockRepo(ctrl)
	mockPermittableRepo := permittable.NewMockRepo(ctrl)
	mockPermittableRepo.EXPECT().
		FindByUserID(gomock.Any(), uid).
		Return(nil, rerror.ErrNotFound)

	c := NewChecker(newTestClient(t), mockRoleRepo, mockPermittableRepo)

	allowed, err := c.Allowed(context.Background(), uid, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

// A nil permittable with no error must also deny.
func TestChecker_Allowed_NilPermittableDenies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uid := user.NewID()
	mockRoleRepo := role.NewMockRepo(ctrl)
	mockPermittableRepo := permittable.NewMockRepo(ctrl)
	mockPermittableRepo.EXPECT().
		FindByUserID(gomock.Any(), uid).
		Return(nil, nil)

	c := NewChecker(newTestClient(t), mockRoleRepo, mockPermittableRepo)

	allowed, err := c.Allowed(context.Background(), uid, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

// A non-not-found error from the permittable repo must be propagated, not
// swallowed into an allow/deny decision.
func TestChecker_Allowed_PermittableRepoErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uid := user.NewID()
	wantErr := errors.New("db down")
	mockRoleRepo := role.NewMockRepo(ctrl)
	mockPermittableRepo := permittable.NewMockRepo(ctrl)
	mockPermittableRepo.EXPECT().
		FindByUserID(gomock.Any(), uid).
		Return(nil, wantErr)

	c := NewChecker(newTestClient(t), mockRoleRepo, mockPermittableRepo)

	allowed, err := c.Allowed(context.Background(), uid, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.ErrorIs(t, err, wantErr)
	assert.False(t, allowed)
}

// An error while resolving the caller's roles must be propagated.
func TestChecker_Allowed_RoleRepoErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uid := user.NewID()
	r := role.New().NewID().Name("role1").MustBuild()
	pmt := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{r.ID()}).
		MustBuild()
	wantErr := errors.New("db down")

	mockRoleRepo := role.NewMockRepo(ctrl)
	mockPermittableRepo := permittable.NewMockRepo(ctrl)
	mockPermittableRepo.EXPECT().
		FindByUserID(gomock.Any(), uid).
		Return(pmt, nil)
	mockRoleRepo.EXPECT().
		FindByIDs(gomock.Any(), pmt.RoleIDs()).
		Return(nil, wantErr)

	c := NewChecker(newTestClient(t), mockRoleRepo, mockPermittableRepo)

	allowed, err := c.Allowed(context.Background(), uid, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.ErrorIs(t, err, wantErr)
	assert.False(t, allowed)
}
