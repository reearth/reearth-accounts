package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspace_ServiceCallerIsJudgedByIntegrationRole(t *testing.T) {
	ctx := context.Background()
	db := memory.New()
	owner := id.NewUserID()
	iid := id.NewIntegrationID()
	owned := workspace.New().NewID().Name("owned").Alias("owned").
		Members(map[workspace.UserID]workspace.Member{owner: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{iid: {Role: role.RoleOwner, InvitedBy: owner}}).
		MustBuild()
	written := workspace.New().NewID().Name("written").Alias("written").
		Members(map[workspace.UserID]workspace.Member{owner: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{iid: {Role: role.RoleWriter, InvitedBy: owner}}).
		MustBuild()
	require.NoError(t, db.Workspace.SaveAll(ctx, workspace.List{owned, written}))

	op := &workspace.Operator{
		Integration:        &iid,
		OwningWorkspaces:   id.WorkspaceIDList{owned.ID()},
		WritableWorkspaces: id.WorkspaceIDList{written.ID()},
	}
	// Cerbos would allow everything; a service caller must not be sent to it.
	uc := NewWorkspace(db, nil, &fakeCerbos{allowed: true})

	got, err := uc.Update(ctx, interfaces.UpdateWorkspaceParam{ID: owned.ID(), Name: lo.ToPtr("renamed"), Alias: lo.ToPtr("renamed-alias")}, op)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name())

	_, err = uc.Update(ctx, interfaces.UpdateWorkspaceParam{ID: written.ID(), Name: lo.ToPtr("nope")}, op)
	assert.ErrorIs(t, err, interfaces.ErrOperationDenied)

	_, err = uc.UpdateUserMember(ctx, written.ID(), owner, role.RoleReader, op)
	assert.Error(t, err)

	_, err = uc.Create(ctx, "new", "new", "", owner, false, op)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator, "creating records a user")
}

func TestWorkspace_ReadsForAClientActingForAUserAreScoped(t *testing.T) {
	ctx := context.Background()
	db := memory.New()
	subject, other := id.NewUserID(), id.NewUserID()
	member := func(name string, users ...id.UserID) *workspace.Workspace {
		m := map[workspace.UserID]workspace.Member{}
		for _, u := range users {
			m[u] = workspace.Member{Role: role.RoleOwner}
		}
		return workspace.New().NewID().Name(name).Alias(name).Members(m).MustBuild()
	}
	mine, shared, theirs := member("mine", subject), member("shared", subject, other), member("theirs", other)
	require.NoError(t, db.Workspace.SaveAll(ctx, workspace.List{mine, shared, theirs}))
	uc := NewWorkspace(db, nil, &fakeCerbos{allowed: true})

	own := &workspace.Operator{User: &subject, OwningWorkspaces: id.WorkspaceIDList{mine.ID(), shared.ID()}}
	client := *own
	client.Client = &workspace.Client{ID: "svc", Name: "svc"}

	ws, err := uc.Fetch(ctx, id.WorkspaceIDList{mine.ID(), theirs.ID()}, &client)
	require.NoError(t, err)
	assert.ElementsMatch(t, id.WorkspaceIDList{mine.ID()}, ws.IDs())
	_, err = uc.FetchByID(ctx, theirs.ID(), &client)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	_, err = uc.FetchByName(ctx, "theirs", &client)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	_, err = uc.FetchByAlias(ctx, "theirs", &client)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	got, err := uc.FetchByID(ctx, shared.ID(), &client)
	require.NoError(t, err)
	assert.Equal(t, shared.ID(), got.ID())

	ws, err = uc.FindByUser(ctx, subject, &client)
	require.NoError(t, err)
	assert.ElementsMatch(t, id.WorkspaceIDList{mine.ID(), shared.ID()}, ws.IDs())
	_, err = uc.FindByUser(ctx, other, &client)
	assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)
	_, err = uc.FetchByUserWithPagination(ctx, other, interfaces.FetchByUserWithPaginationParam{Page: 1, Size: 10}, &client)
	assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)

	t.Run("the user's own token is unscoped as before", func(t *testing.T) {
		got, err := uc.FetchByID(ctx, theirs.ID(), own)
		require.NoError(t, err)
		assert.Equal(t, theirs.ID(), got.ID())
		ws, err := uc.Fetch(ctx, id.WorkspaceIDList{mine.ID(), theirs.ID()}, own)
		require.NoError(t, err)
		assert.Len(t, ws, 2)
		ws, err = uc.FindByUser(ctx, other, own)
		require.NoError(t, err)
		assert.ElementsMatch(t, id.WorkspaceIDList{shared.ID()}, ws.IDs())
	})
}
