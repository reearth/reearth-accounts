package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seederBulkPermittable seeds a workspace owned by a single user plus a set of additional users
// that are not yet members, returning their IDs for use in tests.
func seederBulkPermittable(ownerID user.ID, wsID workspace.ID, extraUsers []user.ID) Seeder {
	return func(ctx context.Context, r *repo.Container) error {
		for _, roleName := range []string{"owner", "maintainer", "writer", "reader", "self"} {
			if err := r.Role.Save(ctx, *role.New().NewID().Name(roleName).MustBuild()); err != nil {
				return err
			}
		}

		owner := user.New().ID(ownerID).Name("owner").Email("owner@bulk.test").MustBuild()
		if err := r.User.Save(ctx, owner); err != nil {
			return err
		}

		for i, uid := range extraUsers {
			u := user.New().ID(uid).
				Name(fmt.Sprintf("extra%d", i)).
				Email(fmt.Sprintf("extra%d@bulk.test", i)).
				MustBuild()
			if err := r.User.Save(ctx, u); err != nil {
				return err
			}
		}

		ownerRole, err := r.Role.FindByName(ctx, "owner")
		if err != nil {
			return err
		}

		ws := workspace.New().ID(wsID).Name("bulk-ws").
			Members(map[user.ID]workspace.Member{
				ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
			}).
			Personal(false).MustBuild()
		if err := r.Workspace.Save(ctx, ws); err != nil {
			return err
		}

		p, err := permittable.New().NewID().UserID(ownerID).Build()
		if err != nil {
			return err
		}
		p.UpdateWorkspaceRole(wsID, ownerRole.ID())
		return r.Permittable.Save(ctx, *p)
	}
}

// TestAddUsersToWorkspace_BulkCreatesPermittables verifies that adding multiple users in a
// single addUsersToWorkspace call creates a permittable for every invited user.
func TestAddUsersToWorkspace_BulkCreatesPermittables(t *testing.T) {
	ownerID := id.NewUserID()
	wsID := id.NewWorkspaceID()
	extraIDs := []user.ID{id.NewUserID(), id.NewUserID(), id.NewUserID()}

	e, r := StartServer(t, &app.Config{}, false,
		seederBulkPermittable(ownerID, wsID, extraIDs))

	ctx := context.Background()

	users := fmt.Sprintf(
		`{userId: "%s", role: writer}, {userId: "%s", role: reader}, {userId: "%s", role: maintainer}`,
		extraIDs[0], extraIDs[1], extraIDs[2],
	)
	query := fmt.Sprintf(
		`mutation { addUsersToWorkspace(input: {workspaceId: "%s", users: [%s]}){ workspace{ id } }}`,
		wsID, users,
	)
	body, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)

	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", ownerID.String()).
		WithBytes(body).
		Expect().Status(http.StatusOK)

	writerRole, err := r.Role.FindByName(ctx, "writer")
	require.NoError(t, err)
	readerRole, err := r.Role.FindByName(ctx, "reader")
	require.NoError(t, err)
	maintainerRole, err := r.Role.FindByName(ctx, "maintainer")
	require.NoError(t, err)

	expected := map[user.ID]id.RoleID{
		extraIDs[0]: writerRole.ID(),
		extraIDs[1]: readerRole.ID(),
		extraIDs[2]: maintainerRole.ID(),
	}

	for uid, wantRoleID := range expected {
		p, err := r.Permittable.FindByUserID(ctx, uid)
		require.NoError(t, err, "permittable missing for user %s", uid)
		require.NotNil(t, p)

		wrs := p.WorkspaceRoles()
		require.Len(t, wrs, 1, "expected exactly one workspace role for user %s", uid)
		assert.Equal(t, wsID, wrs[0].ID())
		assert.Equal(t, wantRoleID, wrs[0].RoleID())
	}
}

// TestAddUsersToWorkspace_BulkPreservesExistingPermittables verifies that adding a user who
// already has permittables for other workspaces does not remove those existing workspace roles.
func TestAddUsersToWorkspace_BulkPreservesExistingPermittables(t *testing.T) {
	ownerID := id.NewUserID()
	wsID := id.NewWorkspaceID()
	otherWsID := id.NewWorkspaceID()
	inviteeID := id.NewUserID()

	e, r := StartServer(t, &app.Config{}, false,
		func(ctx context.Context, repos *repo.Container) error {
			if err := seederBulkPermittable(ownerID, wsID, []user.ID{inviteeID})(ctx, repos); err != nil {
				return err
			}

			// Give the invitee a pre-existing role on a different workspace.
			maintainerRole, err := repos.Role.FindByName(ctx, "maintainer")
			if err != nil {
				return err
			}
			existing, err := permittable.New().NewID().UserID(inviteeID).
				WorkspaceRoles([]permittable.WorkspaceRole{
					permittable.NewWorkspaceRole(otherWsID, maintainerRole.ID()),
				}).Build()
			if err != nil {
				return err
			}
			return repos.Permittable.Save(ctx, *existing)
		},
	)

	ctx := context.Background()

	query := fmt.Sprintf(
		`mutation { addUsersToWorkspace(input: {workspaceId: "%s", users: [{userId: "%s", role: writer}]}){ workspace{ id } }}`,
		wsID, inviteeID,
	)
	body, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)

	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", ownerID.String()).
		WithBytes(body).
		Expect().Status(http.StatusOK)

	maintainerRole, err := r.Role.FindByName(ctx, "maintainer")
	require.NoError(t, err)
	writerRole, err := r.Role.FindByName(ctx, "writer")
	require.NoError(t, err)

	p, err := r.Permittable.FindByUserID(ctx, inviteeID)
	require.NoError(t, err)
	require.NotNil(t, p)

	wrs := p.WorkspaceRoles()
	assert.Len(t, wrs, 2, "both workspace roles should be present")

	roleByWS := make(map[workspace.ID]id.RoleID, len(wrs))
	for _, wr := range wrs {
		roleByWS[wr.ID()] = wr.RoleID()
	}
	assert.Equal(t, maintainerRole.ID(), roleByWS[otherWsID], "pre-existing workspace role must not be removed")
	assert.Equal(t, writerRole.ID(), roleByWS[wsID], "new workspace role must be set")
}

// TestAddUsersToWorkspace_BulkLargeBatch verifies correctness when many users are added in one call.
func TestAddUsersToWorkspace_BulkLargeBatch(t *testing.T) {
	const batchSize = 20

	ownerID := id.NewUserID()
	wsID := id.NewWorkspaceID()
	extraIDs := make([]user.ID, batchSize)
	for i := range extraIDs {
		extraIDs[i] = id.NewUserID()
	}

	e, r := StartServer(t, &app.Config{}, false,
		seederBulkPermittable(ownerID, wsID, extraIDs))

	ctx := context.Background()

	memberInputs := ""
	for i, uid := range extraIDs {
		if i > 0 {
			memberInputs += ", "
		}
		memberInputs += fmt.Sprintf(`{userId: "%s", role: reader}`, uid)
	}
	query := fmt.Sprintf(
		`mutation { addUsersToWorkspace(input: {workspaceId: "%s", users: [%s]}){ workspace{ id } }}`,
		wsID, memberInputs,
	)
	body, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)

	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", ownerID.String()).
		WithBytes(body).
		Expect().Status(http.StatusOK)

	readerRole, err := r.Role.FindByName(ctx, "reader")
	require.NoError(t, err)

	ws, err := r.Workspace.FindByID(ctx, wsID)
	require.NoError(t, err)
	assert.Len(t, ws.Members().Users(), batchSize+1, "workspace should have all invited users plus the owner")

	for _, uid := range extraIDs {
		p, err := r.Permittable.FindByUserID(ctx, uid)
		require.NoError(t, err, "permittable missing for user %s", uid)
		wrs := p.WorkspaceRoles()
		require.Len(t, wrs, 1)
		assert.Equal(t, wsID, wrs[0].ID())
		assert.Equal(t, readerRole.ID(), wrs[0].RoleID())
	}
}
