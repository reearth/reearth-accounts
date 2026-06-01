package pgdoc_test

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/policy"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subsOf(u *user.User) []string {
	out := []string{}
	for _, a := range u.Auths() {
		out = append(out, a.Sub)
	}
	return out
}

func TestUserRoundTrip(t *testing.T) {
	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u, err := user.New().ID(uid).Name("alice").Email("a@example.com").
		Workspace(wid).Alias("alice").Auths([]user.Auth{user.AuthFrom("sub-1")}).Build()
	require.NoError(t, err)

	got, err := pgdoc.NewUserRow(u).Model()
	require.NoError(t, err)
	assert.Equal(t, uid, got.ID())
	assert.Equal(t, "alice", got.Name())
	assert.Equal(t, "a@example.com", got.Email())
	assert.Equal(t, "alice", got.Alias())
	assert.Equal(t, wid, got.Workspace())
	assert.Equal(t, []string{"sub-1"}, subsOf(got))
}

func TestWorkspaceRoundTrip(t *testing.T) {
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	ws, err := workspace.New().NewID().Name("team").Alias("team").Email("t@example.com").
		Members(map[id.UserID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: {Role: role.RoleOwner, InvitedBy: uid}}).
		Build()
	require.NoError(t, err)

	row, members, integrations := pgdoc.NewWorkspaceRows(ws)
	require.NotEmpty(t, row.MembersHash)
	got, err := pgdoc.WorkspaceModel(row, members, integrations)
	require.NoError(t, err)
	assert.Equal(t, ws.ID(), got.ID())
	assert.Equal(t, "team", got.Name())
	assert.Equal(t, "team", got.Alias())
	assert.Contains(t, got.Members().Users(), uid)
	assert.Contains(t, got.Members().Integrations(), iid)
}

func TestRoleRoundTrip(t *testing.T) {
	rl, err := role.New().NewID().Name("admin").Build()
	require.NoError(t, err)
	got, err := pgdoc.NewRoleRow(*rl).Model()
	require.NoError(t, err)
	assert.Equal(t, rl.ID(), got.ID())
	assert.Equal(t, "admin", got.Name())
}

func TestPermittableRoundTrip(t *testing.T) {
	uid := id.NewUserID()
	rid := id.NewRoleID()
	wid := id.NewWorkspaceID()
	p, err := permittable.New().NewID().UserID(uid).
		RoleIDs(id.RoleIDList{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{permittable.NewWorkspaceRole(wid, rid)}).
		Build()
	require.NoError(t, err)

	row, wrs := pgdoc.NewPermittableRow(*p)
	require.Len(t, wrs, 1)
	got, err := pgdoc.PermittableModel(row, wrs)
	require.NoError(t, err)
	assert.Equal(t, uid, got.UserID())
	assert.Equal(t, []id.RoleID{rid}, got.RoleIDs())
	require.Len(t, got.WorkspaceRoles(), 1)
	gotWR := got.WorkspaceRoles()[0]
	assert.Equal(t, wid, gotWR.ID())
	assert.Equal(t, rid, gotWR.RoleID())
}

func TestConfigRoundTrip(t *testing.T) {
	pid := policy.ID("policy-1")
	cfg := config.Config{Migration: 5, Auth: &config.Auth{Cert: "cert", Key: "key"}, DefaultPolicy: &pid}
	got := pgdoc.NewConfigRow(cfg).Model()
	assert.Equal(t, int64(5), got.Migration)
	require.NotNil(t, got.Auth)
	assert.Equal(t, "cert", got.Auth.Cert)
	assert.Equal(t, "key", got.Auth.Key)
	require.NotNil(t, got.DefaultPolicy)
	assert.Equal(t, pid, *got.DefaultPolicy)
}
