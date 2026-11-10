package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/rbac"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionCerbos allows exactly the listed actions, mirroring how the real policy
// grants the member actions to maintainers but transfer_ownership only to owners.
type actionCerbos map[string]bool

func (a actionCerbos) CheckPermission(_ context.Context, _ user.ID, p interfaces.CheckPermissionParam) (*interfaces.CheckPermissionResult, error) {
	return &interfaces.CheckPermissionResult{Allowed: a[p.Action]}, nil
}

var maintainerCerbos = actionCerbos{
	rbac.ActionAddMember:    true,
	rbac.ActionEditMember:   true,
	rbac.ActionDeleteMember: true,
}

type ownersFixture struct {
	db     *repo.Container
	wid    workspace.ID
	owner  user.ID
	owner2 user.ID
	maint  user.ID
	reader user.ID
	newbie user.ID
}

// newOwnersFixture seeds a team workspace. With twoOwners false, owner2 is not a member.
func newOwnersFixture(t *testing.T, twoOwners bool) ownersFixture {
	t.Helper()
	ctx := context.Background()
	f := ownersFixture{
		db:     memory.New(),
		wid:    id.NewWorkspaceID(),
		owner:  id.NewUserID(),
		owner2: id.NewUserID(),
		maint:  id.NewUserID(),
		reader: id.NewUserID(),
	}
	for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
		require.NoError(t, f.db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild()))
	}
	members := map[user.ID]workspace.Member{
		f.owner:  {Role: role.RoleOwner},
		f.maint:  {Role: role.RoleMaintainer},
		f.reader: {Role: role.RoleReader},
	}
	if twoOwners {
		members[f.owner2] = workspace.Member{Role: role.RoleOwner}
	}
	ws := workspace.New().ID(f.wid).Name("team").Alias("team").Members(members).Personal(false).MustBuild()
	require.NoError(t, f.db.Workspace.Save(ctx, ws))
	newbie := user.New().NewID().Name("newbie").Email("newbie@example.com").MustBuild()
	require.NoError(t, f.db.User.Save(ctx, newbie))
	f.newbie = newbie.ID()
	return f
}

func (f ownersFixture) ownerOp(u user.ID) *workspace.Operator {
	return &workspace.Operator{User: lo.ToPtr(u), OwningWorkspaces: []workspace.ID{f.wid}}
}

func (f ownersFixture) maintainerOp() *workspace.Operator {
	return &workspace.Operator{User: lo.ToPtr(f.maint), MaintainableWorkspaces: []workspace.ID{f.wid}}
}

// strangerOp is a caller with no workspace role, for use with a Cerbos that allows everything.
func strangerOp() *workspace.Operator {
	return &workspace.Operator{User: lo.ToPtr(id.NewUserID())}
}

func (f ownersFixture) members(t *testing.T) *workspace.Members {
	t.Helper()
	ws, err := f.db.Workspace.FindByID(context.Background(), f.wid)
	require.NoError(t, err)
	return ws.Members()
}

func TestWorkspace_SeveralOwners_OnlyOwnerManagesOwners(t *testing.T) {
	ctx := context.Background()

	t.Run("owner adds a second owner", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, nil).AddUserMember(ctx, f.wid, map[user.ID]role.RoleType{f.newbie: role.RoleOwner}, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.Equal(t, role.RoleOwner, f.members(t).UserRole(f.newbie))
	})

	t.Run("owner promotes a member to owner", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, nil).UpdateUserMember(ctx, f.wid, f.reader, role.RoleOwner, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.Equal(t, role.RoleOwner, f.members(t).UserRole(f.reader))
	})

	for _, tc := range []struct {
		name   string
		cerbos interfaces.Cerbos
		want   error
	}{
		{name: "without Cerbos", want: interfaces.ErrOperationDenied},
		{name: "with Cerbos granting member actions to maintainers", cerbos: maintainerCerbos, want: interfaces.ErrPermissionDenied},
	} {
		t.Run("maintainer "+tc.name, func(t *testing.T) {
			t.Run("cannot add an owner", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				_, err := NewWorkspace(f.db, nil, tc.cerbos).AddUserMember(ctx, f.wid, map[user.ID]role.RoleType{f.newbie: role.RoleOwner}, f.maintainerOp())
				assert.ErrorIs(t, err, tc.want)
				assert.False(t, f.members(t).HasUser(f.newbie))
			})

			t.Run("cannot promote a member to owner", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				_, err := NewWorkspace(f.db, nil, tc.cerbos).UpdateUserMember(ctx, f.wid, f.reader, role.RoleOwner, f.maintainerOp())
				assert.ErrorIs(t, err, tc.want)
				assert.Equal(t, role.RoleReader, f.members(t).UserRole(f.reader))
			})

			t.Run("cannot demote an owner", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				_, err := NewWorkspace(f.db, nil, tc.cerbos).UpdateUserMember(ctx, f.wid, f.owner2, role.RoleReader, f.maintainerOp())
				assert.ErrorIs(t, err, tc.want)
				assert.Equal(t, role.RoleOwner, f.members(t).UserRole(f.owner2))
			})

			t.Run("cannot remove an owner", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				_, err := NewWorkspace(f.db, nil, tc.cerbos).RemoveUserMember(ctx, f.wid, f.owner2, f.maintainerOp())
				assert.ErrorIs(t, err, tc.want)
				assert.True(t, f.members(t).HasUser(f.owner2))
			})

			t.Run("cannot remove an owner among several members", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				_, err := NewWorkspace(f.db, nil, tc.cerbos).RemoveMultipleUserMembers(ctx, f.wid, workspace.UserIDList{f.reader, f.owner2}, f.maintainerOp())
				assert.ErrorIs(t, err, tc.want)
				assert.True(t, f.members(t).HasUser(f.owner2))
				assert.True(t, f.members(t).HasUser(f.reader))
			})

			t.Run("still manages non-owner members", func(t *testing.T) {
				f := newOwnersFixture(t, true)
				uc := NewWorkspace(f.db, nil, tc.cerbos)
				_, err := uc.AddUserMember(ctx, f.wid, map[user.ID]role.RoleType{f.newbie: role.RoleWriter}, f.maintainerOp())
				require.NoError(t, err)
				_, err = uc.UpdateUserMember(ctx, f.wid, f.reader, role.RoleWriter, f.maintainerOp())
				require.NoError(t, err)
				_, err = uc.RemoveUserMember(ctx, f.wid, f.newbie, f.maintainerOp())
				require.NoError(t, err)
			})
		})
	}

	t.Run("owner demotes another owner", func(t *testing.T) {
		f := newOwnersFixture(t, true)
		_, err := NewWorkspace(f.db, nil, nil).UpdateUserMember(ctx, f.wid, f.owner2, role.RoleMaintainer, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.Equal(t, role.RoleMaintainer, f.members(t).UserRole(f.owner2))
	})

	t.Run("owner removes another owner", func(t *testing.T) {
		f := newOwnersFixture(t, true)
		_, err := NewWorkspace(f.db, nil, nil).RemoveUserMember(ctx, f.wid, f.owner2, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.False(t, f.members(t).HasUser(f.owner2))
	})
}

func TestWorkspace_SeveralOwners_KeepsOneOwner(t *testing.T) {
	ctx := context.Background()
	allowAll := &fakeCerbos{allowed: true}

	t.Run("owner demotes themselves while another owner remains", func(t *testing.T) {
		f := newOwnersFixture(t, true)
		_, err := NewWorkspace(f.db, nil, nil).UpdateUserMember(ctx, f.wid, f.owner, role.RoleMaintainer, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.Equal(t, role.RoleMaintainer, f.members(t).UserRole(f.owner))
	})

	t.Run("owner leaves while another owner remains", func(t *testing.T) {
		f := newOwnersFixture(t, true)
		_, err := NewWorkspace(f.db, nil, nil).RemoveUserMember(ctx, f.wid, f.owner, f.ownerOp(f.owner))
		require.NoError(t, err)
		assert.False(t, f.members(t).HasUser(f.owner))
	})

	t.Run("sole owner cannot demote themselves", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, nil).UpdateUserMember(ctx, f.wid, f.owner, role.RoleMaintainer, f.ownerOp(f.owner))
		assert.ErrorIs(t, err, interfaces.ErrCannotChangeOwnerRole)
		assert.Equal(t, role.RoleOwner, f.members(t).UserRole(f.owner))
	})

	t.Run("sole owner cannot leave", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, nil).RemoveUserMember(ctx, f.wid, f.owner, f.ownerOp(f.owner))
		assert.ErrorIs(t, err, interfaces.ErrOwnerCannotLeaveTheWorkspace)
		assert.True(t, f.members(t).HasUser(f.owner))
	})

	t.Run("a caller Cerbos allows cannot demote the sole owner", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, allowAll).UpdateUserMember(ctx, f.wid, f.owner, role.RoleReader, strangerOp())
		assert.ErrorIs(t, err, interfaces.ErrCannotChangeOwnerRole)
		assert.Equal(t, role.RoleOwner, f.members(t).UserRole(f.owner))
	})

	t.Run("a caller Cerbos allows cannot remove the sole owner", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, allowAll).RemoveUserMember(ctx, f.wid, f.owner, strangerOp())
		assert.ErrorIs(t, err, interfaces.ErrOwnerCannotLeaveTheWorkspace)
		assert.True(t, f.members(t).HasUser(f.owner))
	})

	t.Run("removing every owner at once fails", func(t *testing.T) {
		f := newOwnersFixture(t, true)
		_, err := NewWorkspace(f.db, nil, nil).RemoveMultipleUserMembers(ctx, f.wid, workspace.UserIDList{f.owner, f.owner2}, f.ownerOp(f.owner))
		assert.ErrorIs(t, err, interfaces.ErrOwnerCannotLeaveTheWorkspace)
	})
}

func TestWorkspace_SeveralOwners_IntegrationCannotBeOwner(t *testing.T) {
	ctx := context.Background()

	t.Run("add", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		_, err := NewWorkspace(f.db, nil, nil).AddIntegrationMember(ctx, f.wid, id.NewIntegrationID(), role.RoleOwner, f.ownerOp(f.owner))
		assert.ErrorIs(t, err, workspace.ErrCannotChangeRoleToOwner)
		assert.Empty(t, f.members(t).Integrations())
	})

	t.Run("update", func(t *testing.T) {
		f := newOwnersFixture(t, false)
		iid := id.NewIntegrationID()
		uc := NewWorkspace(f.db, nil, nil)
		_, err := uc.AddIntegrationMember(ctx, f.wid, iid, role.RoleWriter, f.ownerOp(f.owner))
		require.NoError(t, err)
		_, err = uc.UpdateIntegration(ctx, f.wid, iid, role.RoleOwner, f.ownerOp(f.owner))
		assert.ErrorIs(t, err, workspace.ErrCannotChangeRoleToOwner)
		assert.Equal(t, role.RoleWriter, f.members(t).Integration(iid).Role)
	})
}
