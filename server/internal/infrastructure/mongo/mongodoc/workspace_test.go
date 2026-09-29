package mongodoc

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceDocument_RoundTrip_ScimConfigAndExternalID(t *testing.T) {
	uid := id.NewUserID()

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	cfg.SetTokenHash("bcrypt-hash")
	cfg.SetGroupRoleMapping(map[string]role.RoleType{"admins": role.RoleOwner})

	ws, err := workspace.New().NewID().Name("team").Alias("team").Email("t@example.com").
		Members(map[id.UserID]workspace.Member{
			uid: {Role: role.RoleOwner, InvitedBy: uid, ExternalID: "ext-456"},
		}).
		ScimConfig(cfg).
		Build()
	require.NoError(t, err)

	doc, _ := NewWorkspace(ws)

	// ScimConfig persisted
	require.NotNil(t, doc.ScimConfig)
	assert.True(t, doc.ScimConfig.Enabled)
	assert.Equal(t, "bcrypt-hash", doc.ScimConfig.TokenHash)
	assert.Equal(t, map[string]string{"admins": string(role.RoleOwner)}, doc.ScimConfig.GroupRoleMapping)

	// ExternalID persisted on member sub-doc
	require.Contains(t, doc.Members, uid.String())
	assert.Equal(t, "ext-456", doc.Members[uid.String()].ExternalID)

	// Hydrate back to domain
	got, err := doc.Model()
	require.NoError(t, err)

	gotCfg := got.ScimConfig()
	require.NotNil(t, gotCfg)
	assert.True(t, gotCfg.Enabled())
	assert.Equal(t, "bcrypt-hash", gotCfg.TokenHash())
	assert.Equal(t, map[string]role.RoleType{"admins": role.RoleOwner}, gotCfg.GroupRoleMapping())

	assert.Equal(t, "ext-456", got.Members().Users()[uid].ExternalID)
}

func TestWorkspaceDocument_RoundTrip_ClearScimConfig(t *testing.T) {
	uid := id.NewUserID()

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	cfg.SetTokenHash("old-hash")

	ws, err := workspace.New().NewID().Name("team").Alias("team").Email("t@example.com").
		Members(map[id.UserID]workspace.Member{
			uid: {Role: role.RoleOwner, InvitedBy: uid},
		}).
		ScimConfig(cfg).
		Build()
	require.NoError(t, err)

	// Clear the SCIM config
	ws.SetScimConfig(nil)

	doc, _ := NewWorkspace(ws)
	assert.Nil(t, doc.ScimConfig)

	got, err := doc.Model()
	require.NoError(t, err)
	assert.Nil(t, got.ScimConfig())
}
