package httpmodel

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkspaceResponse_DropsMissingUsers(t *testing.T) {
	ownerUser := user.New().NewID().Name("owner").Email("owner@example.com").Workspace(workspace.NewID()).MustBuild()
	owner, ghost := ownerUser.ID(), user.NewID()
	integration := workspace.NewIntegrationID()
	w := workspace.New().NewID().Name("team").Alias("team").
		Members(map[user.ID]workspace.Member{
			owner: {Role: role.RoleOwner, InvitedBy: owner},
			ghost: {Role: role.RoleWriter, InvitedBy: owner},
		}).
		Integrations(map[workspace.IntegrationID]workspace.Member{
			integration: {Role: role.RoleReader, InvitedBy: owner},
		}).
		MustBuild()

	got := NewWorkspaceResponse(w, map[user.ID]*user.User{owner: ownerUser}, MemberUsers{})
	require.NotNil(t, got)
	assert.ElementsMatch(t, []WorkspaceMemberResponse{
		{UserID: owner.String(), Role: "owner"},
		{IntegrationID: integration.String(), Role: "reader"},
	}, got.Members)
}
