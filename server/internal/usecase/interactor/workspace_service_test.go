package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
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
