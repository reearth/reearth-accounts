package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	infraCerbos "github.com/reearth/reearth-accounts/internal/infrastructure/cerbos"
	"github.com/reearth/reearth-accounts/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway/mock_gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/internal/usecase/repo/mock_repo"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewCerbos(t *testing.T) {
	memory := memory.New()

	cerbosClient, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
	if err != nil {
		t.Fatal(err)
	}
	cerbosAdapter := infraCerbos.NewCerbosAdapter(cerbosClient)

	c := NewCerbos(cerbosAdapter, memory)
	assert.NotNil(t, c)
}

func TestCheckPermission(t *testing.T) {
	// prepare
	ctx := context.Background()
	uid := user.NewID()
	r := role.New().NewID().Name("role1").MustBuild()
	rs := role.List{r}
	p := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{r.ID()}).
		MustBuild()
	param := interfaces.CheckPermissionParam{
		Service:  "service",
		Resource: "resource",
		Action:   "read",
	}

	// create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoleRepo := mock_repo.NewMockRole(ctrl)
	mockPermittableRepo := mock_repo.NewMockPermittable(ctrl)
	mockCerbos := mock_gateway.NewMockCerbosGateway(ctrl)

	c := &Cerbos{
		roleRepo:        mockRoleRepo,
		permittableRepo: mockPermittableRepo,
		cerbos:          mockCerbos,
	}

	t.Run("Action allowed", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&cerbos.CheckResourcesResponse{
				CheckResourcesResponse: &responsev1.CheckResourcesResponse{
					Results: []*responsev1.CheckResourcesResponse_ResultEntry{
						{
							Actions: map[string]effectv1.Effect{
								"read": effectv1.Effect_EFFECT_ALLOW,
							},
						},
					},
				},
			}, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.True(t, res.Allowed)
	})

	t.Run("Action denied", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&cerbos.CheckResourcesResponse{
				CheckResourcesResponse: &responsev1.CheckResourcesResponse{
					Results: []*responsev1.CheckResourcesResponse_ResultEntry{
						{
							Actions: map[string]effectv1.Effect{
								"read": effectv1.Effect_EFFECT_DENY,
							},
						},
					},
				},
			}, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("permittabl.FindByUserID returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(nil, errors.New("db error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("permittabl.FindByUserID returns nil permittable", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(nil, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("role.FindByIDs returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(nil, errors.New("db error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("CheckPermissions returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("cerbos error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("CheckPermissions returns nil response", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.ErrorIs(t, err, interfaces.ErrOperationDenied)
		assert.Nil(t, res)
	})

	t.Run("Action not found in results", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), p.RoleIDs()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&cerbos.CheckResourcesResponse{
				CheckResourcesResponse: &responsev1.CheckResourcesResponse{
					Results: []*responsev1.CheckResourcesResponse_ResultEntry{
						{
							Actions: map[string]effectv1.Effect{
								"edit": effectv1.Effect_EFFECT_ALLOW,
							},
						},
					},
				},
			}, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})
}
