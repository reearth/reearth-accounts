package app

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

func TestRunMigration(t *testing.T) {
	// prepare
	ctx := context.Background()

	uId1 := user.NewID()
	uId2 := user.NewID()
	uId3 := user.NewID()
	u1 := user.New().ID(uId1).Name("user1").Email("user1@test.com").MustBuild()
	u2 := user.New().ID(uId2).Name("user2").Email("user2@test.com").MustBuild()
	u3 := user.New().ID(uId3).Name("user3").Email("user3@test.com").MustBuild()

	iId1 := id.NewIntegrationID()
	iId2 := id.NewIntegrationID()
	iId3 := id.NewIntegrationID()
	iUserId1, err := user.IDFrom(iId1.String())
	if err != nil {
		t.Fatal(err)
	}
	iUserId2, err := user.IDFrom(iId2.String())
	if err != nil {
		t.Fatal(err)
	}
	iUserId3, err := user.IDFrom(iId3.String())
	if err != nil {
		t.Fatal(err)
	}

	roleOwner := workspace.Member{
		Role:      role.RoleOwner,
		InvitedBy: uId1,
	}

	wId1 := workspace.NewID()
	wId2 := workspace.NewID()
	w1 := workspace.New().ID(wId1).
		Name("w1").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId1: roleOwner,
			uId2: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId1: roleOwner,
			iId2: roleOwner,
		}).
		MustBuild()
	w2 := workspace.New().ID(wId2).
		Name("w2").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId3: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId3: roleOwner,
		}).
		MustBuild()

	tests := []struct {
		name    string
		setup   func(ctx context.Context, repos *repo.Container)
		assert  func(t *testing.T, ctx context.Context, repos *repo.Container)
		wantErr bool
	}{
		{
			name: "should create maintainer role and assign it to workspace users and integrations",
			setup: func(ctx context.Context, repos *repo.Container) {
				userRepo := repo.NewMultiUser(memory.NewUserWith(u1, u2, u3))
				workspaceRepo := memory.NewWorkspaceWith(w1, w2)
				repos.User = userRepo
				repos.Workspace = workspaceRepo
			},
			assert: func(t *testing.T, ctx context.Context, repos *repo.Container) {
				assertPermittablesAndRoles(t, ctx, repos, user.IDList{uId1, uId2, uId3, iUserId1, iUserId2, iUserId3})
			},
		},
		{
			name: "should not duplicate maintainer role when it already exists",
			setup: func(ctx context.Context, repos *repo.Container) {
				existingRole, _ := role.New().NewID().Name("maintainer").Build()
				err = repos.Role.Save(ctx, *existingRole)
				if err != nil {
					t.Fatal(err)
				}

				userRepo := repo.NewMultiUser(memory.NewUserWith(u1, u2, u3))
				workspaceRepo := memory.NewWorkspaceWith(w1, w2)
				repos.User = userRepo
				repos.Workspace = workspaceRepo
			},
			assert: func(t *testing.T, ctx context.Context, repos *repo.Container) {
				assertPermittablesAndRoles(t, ctx, repos, user.IDList{uId1, uId2, uId3, iUserId1, iUserId2, iUserId3})
			},
		},
		{
			name: "should not add maintainer role if user already has it",
			setup: func(ctx context.Context, repos *repo.Container) {
				existingRole, _ := role.New().NewID().Name("maintainer").Build()
				err = repos.Role.Save(ctx, *existingRole)
				if err != nil {
					t.Fatal(err)
				}

				p, _ := permittable.New().
					NewID().
					UserID(uId1).
					RoleIDs([]id.RoleID{existingRole.ID()}).
					Build()
				err = repos.Permittable.Save(ctx, *p)
				if err != nil {
					t.Fatal(err)
				}

				userRepo := repo.NewMultiUser(memory.NewUserWith(u1, u2, u3))
				workspaceRepo := memory.NewWorkspaceWith(w1, w2)
				repos.User = userRepo
				repos.Workspace = workspaceRepo
			},
			assert: func(t *testing.T, ctx context.Context, repos *repo.Container) {
				permittable, err := repos.Permittable.FindByUserID(ctx, uId1)
				assert.NoError(t, err)
				assert.Equal(t, 1, len(permittable.RoleIDs()))

				assertPermittablesAndRoles(t, ctx, repos, user.IDList{uId1, uId2, uId3, iUserId1, iUserId2, iUserId3})
			},
		},
		{
			name: "should add maintainer role when user has other roles",
			setup: func(ctx context.Context, repos *repo.Container) {
				otherRole, _ := role.New().NewID().Name("other_role").Build()
				err = repos.Role.Save(ctx, *otherRole)
				if err != nil {
					t.Fatal(err)
				}

				p, _ := permittable.New().
					NewID().
					UserID(uId1).
					RoleIDs([]id.RoleID{otherRole.ID()}).
					Build()
				err = repos.Permittable.Save(ctx, *p)
				if err != nil {
					t.Fatal(err)
				}

				userRepo := repo.NewMultiUser(memory.NewUserWith(u1, u2, u3))
				workspaceRepo := memory.NewWorkspaceWith(w1, w2)
				repos.User = userRepo
				repos.Workspace = workspaceRepo
			},
			assert: func(t *testing.T, ctx context.Context, repos *repo.Container) {
				roles, err := repos.Role.FindAll(ctx)
				assert.NoError(t, err)
				assert.Equal(t, 2, len(roles))

				permittable, err := repos.Permittable.FindByUserID(ctx, uId1)
				assert.NoError(t, err)
				assert.Equal(t, 2, len(permittable.RoleIDs()))

				assertPermittablesAndRoles(t, ctx, repos, user.IDList{uId1, uId2, uId3, iUserId1, iUserId2, iUserId3})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memoryRepo := memory.New()

			if tt.setup != nil {
				tt.setup(ctx, memoryRepo)
			}

			err := runMigration(ctx, memoryRepo)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			if tt.assert != nil {
				tt.assert(t, ctx, memoryRepo)
			}
		})
	}
}

func assertPermittablesAndRoles(t *testing.T, ctx context.Context, repos *repo.Container, expectedUserIDs user.IDList) {
	// role
	roles, err := repos.Role.FindAll(ctx)
	assert.NoError(t, err)
	var maintainerRole *role.Role
	for _, r := range roles {
		if r.Name() == "maintainer" {
			if maintainerRole != nil {
				t.Fatal("maintainer role already exists")
			}
			maintainerRole = r
		}
	}
	assert.NotNil(t, maintainerRole)

	// permittable
	permittables, err := repos.Permittable.FindByUserIDs(ctx, expectedUserIDs)
	assert.NoError(t, err)
	assert.Equal(t, len(expectedUserIDs), len(permittables))

	// userID
	userIds := make(user.IDList, 0, len(permittables))
	for _, p := range permittables {
		userIds = append(userIds, p.UserID())
	}
	for _, expectedID := range expectedUserIDs {
		assert.Contains(t, userIds, expectedID)
	}

	// role assignment
	for _, p := range permittables {
		assert.Contains(t, p.RoleIDs(), maintainerRole.ID())
	}
}
