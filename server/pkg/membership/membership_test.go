package membership

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeMemberRole(t *testing.T) {
	member := user.NewID()
	outsider := user.NewID()
	otherWs := workspace.NewID()
	maintainer := role.New().NewID().Name(string(role.RoleMaintainer)).MustBuild()
	oldRoleID := role.NewID()
	otherRoleID := role.NewID()

	type fixture struct {
		ws *workspace.Workspace
		p  *permittable.Permittable
	}
	newWs := func(personal bool) *workspace.Workspace {
		return workspace.New().NewID().Name("ws").Alias("ws").Personal(personal).
			Members(map[workspace.UserID]workspace.Member{member: {Role: role.RoleWriter}}).
			MustBuild()
	}
	withEntry := func(ws *workspace.Workspace) *permittable.Permittable {
		return permittable.New().NewID().UserID(member).WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(otherWs, otherRoleID),
			permittable.NewWorkspaceRole(ws.ID(), oldRoleID),
		}).MustBuild()
	}

	tests := []struct {
		name    string
		setup   func() fixture
		uid     user.ID
		role    *role.Role
		wantErr error
	}{
		{
			name: "updates both aggregates",
			setup: func() fixture {
				ws := newWs(false)
				return fixture{ws: ws, p: withEntry(ws)}
			},
			uid:  member,
			role: maintainer,
		},
		{
			name: "adds the workspace entry to a permittable without one",
			setup: func() fixture {
				ws := newWs(false)
				p, err := permittable.NewForUser(member)
				require.NoError(t, err)
				return fixture{ws: ws, p: p}
			},
			uid:  member,
			role: maintainer,
		},
		{
			name: "non-member",
			setup: func() fixture {
				ws := newWs(false)
				p, err := permittable.NewForUser(outsider)
				require.NoError(t, err)
				return fixture{ws: ws, p: p}
			},
			uid:     outsider,
			role:    maintainer,
			wantErr: workspace.ErrTargetUserNotInTheWorkspace,
		},
		{
			name: "personal workspace",
			setup: func() fixture {
				ws := newWs(true)
				return fixture{ws: ws, p: withEntry(ws)}
			},
			uid:     member,
			role:    maintainer,
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
		},
		{
			name: "permittable of another user",
			setup: func() fixture {
				ws := newWs(false)
				p, err := permittable.NewForUser(outsider)
				require.NoError(t, err)
				return fixture{ws: ws, p: p}
			},
			uid:     member,
			role:    maintainer,
			wantErr: ErrPermittableUserMismatch,
		},
		{
			name: "role name is not a role type",
			setup: func() fixture {
				ws := newWs(false)
				return fixture{ws: ws, p: withEntry(ws)}
			},
			uid:     member,
			role:    role.New().NewID().Name("superuser").MustBuild(),
			wantErr: role.ErrInvalidRole,
		},
		{
			name: "nil role",
			setup: func() fixture {
				ws := newWs(false)
				return fixture{ws: ws, p: withEntry(ws)}
			},
			uid:     member,
			wantErr: ErrNilRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setup()
			wantMembers := f.ws.Members().Users()
			wantRoles := append([]permittable.WorkspaceRole(nil), f.p.WorkspaceRoles()...)

			err := ChangeMemberRole(f.ws, f.p, tt.uid, tt.role)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, wantMembers, f.ws.Members().Users(), "workspace must be unchanged")
				assert.Equal(t, wantRoles, f.p.WorkspaceRoles(), "permittable must be unchanged")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, role.RoleType(tt.role.Name()), f.ws.Members().UserRole(tt.uid))
			assert.Equal(t, tt.role.ID(), roleIDFor(f.p, f.ws.ID()))
			for _, wr := range wantRoles {
				if wr.ID() != f.ws.ID() {
					assert.Equal(t, wr.RoleID(), roleIDFor(f.p, wr.ID()), "other workspace entries must be kept")
				}
			}
		})
	}

	t.Run("nil workspace or permittable", func(t *testing.T) {
		ws := newWs(false)
		assert.ErrorIs(t, ChangeMemberRole(nil, withEntry(ws), member, maintainer), ErrNilWorkspace)
		assert.ErrorIs(t, ChangeMemberRole(ws, nil, member, maintainer), ErrNilPermittable)
	})
}

func roleIDFor(p *permittable.Permittable, wid workspace.ID) role.ID {
	for _, wr := range p.WorkspaceRoles() {
		if wr.ID() == wid {
			return wr.RoleID()
		}
	}
	return role.ID{}
}
