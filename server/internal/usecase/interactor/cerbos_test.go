package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	infraCerbos "github.com/reearth/reearth-accounts/server/internal/infrastructure/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock_gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo/mock_repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
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

	c := NewCerbos(memory, cerbosAdapter)
	assert.NotNil(t, c)
}

func TestCheckPermission(t *testing.T) {
	// prepare
	ctx := context.Background()
	uid := user.NewID()
	r := role.New().NewID().Name("role1").MustBuild()
	selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
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
	mockWorkspaceRepo := mock_repo.NewMockWorkspace(ctrl)
	mockCerbos := mock_gateway.NewMockCerbosGateway(ctrl)

	c := &Cerbos{
		roleRepo:        mockRoleRepo,
		permittableRepo: mockPermittableRepo,
		workspaceRepo:   mockWorkspaceRepo,
		cerbos:          mockCerbos,
	}

	t.Run("Action allowed", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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

	t.Run("role.FindByName returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(nil, errors.New("db error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("role.FindByIDs returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
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

func TestCheckPermission_WorkspaceResource(t *testing.T) {
	// prepare
	ctx := context.Background()
	uid := user.NewID()
	wid := id.NewWorkspaceID()
	wsAlias := "test-workspace"

	// Create roles
	ownerRole := role.New().NewID().Name("owner").MustBuild()
	selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
	rs := role.List{ownerRole, selfRole}

	// Create permittable with workspace role
	p := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{}).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(wid, ownerRole.ID()),
		}).
		MustBuild()

	// Create workspace
	ws := workspace.New().
		ID(wid).
		Alias(wsAlias).
		MustBuild()

	param := interfaces.CheckPermissionParam{
		Service:        "service",
		Resource:       interfaces.ResourceWorkspace,
		Action:         "read",
		WorkspaceAlias: wsAlias,
	}

	// create mocks
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoleRepo := mock_repo.NewMockRole(ctrl)
	mockPermittableRepo := mock_repo.NewMockPermittable(ctrl)
	mockWorkspaceRepo := mock_repo.NewMockWorkspace(ctrl)
	mockCerbos := mock_gateway.NewMockCerbosGateway(ctrl)

	c := &Cerbos{
		roleRepo:        mockRoleRepo,
		permittableRepo: mockPermittableRepo,
		workspaceRepo:   mockWorkspaceRepo,
		cerbos:          mockCerbos,
	}

	t.Run("Workspace action allowed", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
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

	t.Run("Workspace not found", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(nil, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("User not member of workspace", func(t *testing.T) {
		// Create permittable without workspace roles
		pNoWorkspace := permittable.New().
			NewID().
			UserID(uid).
			RoleIDs([]id.RoleID{}).
			WorkspaceRoles([]permittable.WorkspaceRole{}).
			MustBuild()

		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(pNoWorkspace, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("Workspace alias empty", func(t *testing.T) {
		emptyAliasParam := interfaces.CheckPermissionParam{
			Service:        "service",
			Resource:       interfaces.ResourceWorkspace,
			Action:         "read",
			WorkspaceAlias: "",
		}

		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)

		res, err := c.CheckPermission(ctx, uid, emptyAliasParam)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("Workspace repo returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(nil, errors.New("db error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Workspace action denied by Cerbos", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
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

	t.Run("User with workspace role in different workspace", func(t *testing.T) {
		// Create a different workspace ID
		differentWid := id.NewWorkspaceID()

		// Create permittable with workspace role for different workspace
		pDifferentWs := permittable.New().
			NewID().
			UserID(uid).
			RoleIDs([]id.RoleID{}).
			WorkspaceRoles([]permittable.WorkspaceRole{
				permittable.NewWorkspaceRole(differentWid, ownerRole.ID()),
			}).
			MustBuild()

		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(pDifferentWs, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.False(t, res.Allowed)
	})

	t.Run("Multiple workspace roles for same user in workspace", func(t *testing.T) {
		// Create additional roles
		readerRole := role.New().NewID().Name("reader").MustBuild()
		multiRoles := role.List{ownerRole, readerRole, selfRole}

		// Create permittable with multiple workspace roles for same workspace
		pMultiRoles := permittable.New().
			NewID().
			UserID(uid).
			RoleIDs([]id.RoleID{}).
			WorkspaceRoles([]permittable.WorkspaceRole{
				permittable.NewWorkspaceRole(wid, ownerRole.ID()),
				permittable.NewWorkspaceRole(wid, readerRole.ID()),
			}).
			MustBuild()

		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(pMultiRoles, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, roleIDs id.RoleIDList) {
				// Verify that self role is included in the role IDs
				assert.Contains(t, roleIDs, selfRole.ID())
				// Verify that both workspace roles are included
				assert.Contains(t, roleIDs, ownerRole.ID())
				assert.Contains(t, roleIDs, readerRole.ID())
				// Verify total count (owner + reader + self)
				assert.Len(t, roleIDs, 3)
			}).
			Return(multiRoles, nil)
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

	t.Run("Workspace role.FindByIDs returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("db error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Workspace CheckPermissions returns error", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("cerbos error"))

		res, err := c.CheckPermission(ctx, uid, param)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Workspace CheckPermissions returns nil response", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil)

		res, err := c.CheckPermission(ctx, uid, param)
		assert.ErrorIs(t, err, interfaces.ErrOperationDenied)
		assert.Nil(t, res)
	})

	t.Run("Workspace action not found in results", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&cerbos.CheckResourcesResponse{
				CheckResourcesResponse: &responsev1.CheckResourcesResponse{
					Results: []*responsev1.CheckResourcesResponse_ResultEntry{
						{
							Actions: map[string]effectv1.Effect{
								"write": effectv1.Effect_EFFECT_ALLOW,
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

	t.Run("Self role is properly included with workspace roles", func(t *testing.T) {
		mockPermittableRepo.EXPECT().
			FindByUserID(gomock.Any(), uid).
			Return(p, nil)
		mockRoleRepo.EXPECT().
			FindByName(gomock.Any(), interfaces.RoleSelf).
			Return(selfRole, nil)
		mockWorkspaceRepo.EXPECT().
			FindByAlias(gomock.Any(), wsAlias).
			Return(ws, nil)
		mockRoleRepo.EXPECT().
			FindByIDs(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, roleIDs id.RoleIDList) {
				// Verify that self role is included
				assert.Contains(t, roleIDs, selfRole.ID())
				// Verify that workspace role is included
				assert.Contains(t, roleIDs, ownerRole.ID())
				// Verify count (owner + self = 2)
				assert.Len(t, roleIDs, 2)
			}).
			Return(rs, nil)
		mockCerbos.EXPECT().
			CheckPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, principal *cerbos.Principal, _ []*cerbos.Resource, _ []string) {
				// Verify principal contains both role names
				roles := principal.Roles()
				assert.Contains(t, roles, ownerRole.Name())
				assert.Contains(t, roles, selfRole.Name())
				assert.Len(t, roles, 2)
			}).
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
}
