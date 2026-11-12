package httpmodel

import (
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeUsername(t *testing.T) {
	t.Run("email input is replaced with user- prefixed unique string", func(t *testing.T) {
		got := SanitizeUsername("user@example.com")
		assert.True(t, strings.HasPrefix(got, "user-"), "expected 'user-' prefix, got %q", got)
		assert.Greater(t, len(got), len("user-"), "expected non-empty ULID suffix after prefix")
	})

	t.Run("non-email input is returned unchanged", func(t *testing.T) {
		assert.Equal(t, "alice", SanitizeUsername("alice"))
		assert.Equal(t, "user-", SanitizeUsername("user-"))
	})

	t.Run("two calls with the same email produce different values", func(t *testing.T) {
		a := SanitizeUsername("same@example.com")
		b := SanitizeUsername("same@example.com")
		assert.NotEqual(t, a, b, "each call must generate a fresh unique ULID to avoid workspace alias collisions")
	})

	t.Run("empty string is returned unchanged", func(t *testing.T) {
		assert.Equal(t, "", SanitizeUsername(""))
	})
}

func TestApplyPermittables(t *testing.T) {
	userA := user.NewID()
	userB := user.NewID()
	roleA := id.NewRoleID()
	wsA := workspace.NewID()

	pa := permittable.New().NewID().UserID(userA).
		RoleIDs(id.RoleIDList{roleA}).
		WorkspaceRoles([]permittable.WorkspaceRole{permittable.NewWorkspaceRole(wsA, roleA)}).
		MustBuild()

	t.Run("resolves names when maps are provided", func(t *testing.T) {
		roleNames := map[string]string{roleA.String(): "owner"}
		wsInfo := map[string]WorkspaceInfo{wsA.String(): {Name: "My Workspace", Alias: "my-workspace"}}

		got := []*UserResponse{{MemberUserResponse: MemberUserResponse{ID: userA.String()}}, {MemberUserResponse: MemberUserResponse{ID: userB.String()}}}
		ApplyPermittables(got, permittable.List{pa}, roleNames, wsInfo)

		assert.Equal(t, []string{"owner"}, got[0].PlatformRoles)
		assert.Equal(t, []UserWorkspaceResponse{
			{WorkspaceID: wsA.String(), Name: "My Workspace", Alias: "my-workspace", Role: "owner"},
		}, got[0].Workspaces)

		// userB has no Permittable record: left untouched (nil).
		assert.Nil(t, got[1].PlatformRoles)
		assert.Nil(t, got[1].Workspaces)
	})

	t.Run("falls back to raw ids when maps are nil or missing entries", func(t *testing.T) {
		got := []*UserResponse{{MemberUserResponse: MemberUserResponse{ID: userA.String()}}}
		ApplyPermittables(got, permittable.List{pa}, nil, nil)

		assert.Equal(t, []string{roleA.String()}, got[0].PlatformRoles)
		assert.Equal(t, []UserWorkspaceResponse{
			{WorkspaceID: wsA.String(), Role: roleA.String()},
		}, got[0].Workspaces)
	})
}

func TestNewUserResponse_Email(t *testing.T) {
	alice := user.New().NewID().Name("alice").Email("alice@example.com").Workspace(workspace.NewID()).MustBuild()
	bob := user.New().NewID().Name("bob").Email("bob@example.com").Workspace(workspace.NewID()).MustBuild()
	simpleAlice := &user.Simple{ID: alice.ID(), Name: alice.Name(), Email: alice.Email()}

	tests := []struct {
		name   string
		viewer EmailViewer
		want   string
	}{
		{"owner sees own email", ViewerOf(alice), "alice@example.com"},
		{"other user sees none", ViewerOf(bob), ""},
		{"anonymous sees none", ViewerOf(nil), ""},
		{"zero viewer sees none", EmailViewer{}, ""},
		{"unrestricted route", UnrestrictedViewer(), "alice@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NewUserResponse(alice, tt.viewer).Email)
			assert.Equal(t, tt.want, NewUserResponses(user.List{alice}, tt.viewer)[0].Email)
			assert.Equal(t, tt.want, NewSimpleUserResponse(simpleAlice, tt.viewer).Email)
		})
	}
}
