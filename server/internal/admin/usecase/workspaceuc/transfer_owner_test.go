package workspaceuc

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transferFixture struct {
	wsRepo          workspace.Repo
	userRepo        user.Repo
	roleRepo        *memory.Role
	permittableRepo *memory.Permittable
	tx              *usecasex.NopTransaction
	roles           map[role.RoleType]*role.Role
	uc              *TransferOwnerUseCase
}

func newTransferFixture(t *testing.T, ws *workspace.Workspace, users ...*user.User) *transferFixture {
	t.Helper()
	roles := map[role.RoleType]*role.Role{}
	list := make([]*role.Role, 0, 4)
	for _, rt := range []role.RoleType{role.RoleOwner, role.RoleMaintainer, role.RoleWriter, role.RoleReader} {
		r := role.New().NewID().Name(string(rt)).MustBuild()
		roles[rt] = r
		list = append(list, r)
	}
	f := &transferFixture{
		wsRepo:          memory.NewWorkspaceWith(ws),
		userRepo:        memory.NewUserWith(users...),
		roleRepo:        memory.NewRoleWith(list...),
		permittableRepo: memory.NewPermittable(),
		tx:              &usecasex.NopTransaction{},
		roles:           roles,
	}
	f.uc = NewTransferOwnerUseCase(f.wsRepo, f.userRepo, f.roleRepo, f.permittableRepo, f.tx)
	return f
}

func (f *transferFixture) assertPermittableRole(t *testing.T, uid user.ID, wid workspace.ID, want role.RoleType) {
	t.Helper()
	p, err := f.permittableRepo.FindByUserID(context.Background(), uid)
	require.NoError(t, err)
	for _, wr := range p.WorkspaceRoles() {
		if wr.ID() == wid {
			assert.Equal(t, f.roles[want].ID(), wr.RoleID())
			return
		}
	}
	t.Fatalf("permittable of %s has no role for workspace %s", uid, wid)
}

func (f *transferFixture) assertNoPermittable(t *testing.T, uid user.ID) {
	t.Helper()
	_, err := f.permittableRepo.FindByUserID(context.Background(), uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func newUser(name string) *user.User {
	return user.New().NewID().Name(name).Email(name + "@example.com").MustBuild()
}

func teamWs(members map[workspace.UserID]workspace.Member) *workspace.Workspace {
	return workspace.New().NewID().Name("Team").Alias("team").Members(members).MustBuild()
}

func TestTransferOwner_SoleOwnerToMember(t *testing.T) {
	ctx := context.Background()
	a, b := newUser("a"), newUser("b")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleWriter},
	})
	f := newTransferFixture(t, ws, a, b)

	out, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.NoError(t, err)
	assert.Equal(t, b.ID(), out.NewOwner.ID())
	assert.Equal(t, []user.ID{a.ID()}, out.DemotedUserIDs)
	assert.True(t, f.tx.IsCommitted())

	saved, err := f.wsRepo.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(b.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(a.ID()))
	f.assertPermittableRole(t, b.ID(), ws.ID(), role.RoleOwner)
	f.assertPermittableRole(t, a.ID(), ws.ID(), role.RoleMaintainer)
}

func TestTransferOwner_DemotesEveryOtherOwner(t *testing.T) {
	ctx := context.Background()
	a, b, c := newUser("a"), newUser("b"), newUser("c")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleMaintainer},
		c.ID(): {Role: role.RoleOwner, Disabled: true},
	})
	f := newTransferFixture(t, ws, a, b, c)

	out, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.NoError(t, err)
	assert.ElementsMatch(t, []user.ID{a.ID(), c.ID()}, out.DemotedUserIDs)

	saved, err := f.wsRepo.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, []workspace.UserID{b.ID()}, saved.Members().UsersByRole(role.RoleOwner))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(a.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(c.ID()))
	f.assertPermittableRole(t, a.ID(), ws.ID(), role.RoleMaintainer)
	f.assertPermittableRole(t, c.ID(), ws.ID(), role.RoleMaintainer)
	f.assertPermittableRole(t, b.ID(), ws.ID(), role.RoleOwner)
}

func TestTransferOwner_TargetIsOneOfSeveralOwners(t *testing.T) {
	ctx := context.Background()
	a, b := newUser("a"), newUser("b")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleOwner},
	})
	f := newTransferFixture(t, ws, a, b)

	out, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.NoError(t, err)
	assert.Equal(t, []user.ID{a.ID()}, out.DemotedUserIDs)

	saved, err := f.wsRepo.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(b.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(a.ID()))
	f.assertPermittableRole(t, a.ID(), ws.ID(), role.RoleMaintainer)
	f.assertPermittableRole(t, b.ID(), ws.ID(), role.RoleOwner)
}

func TestTransferOwner_ReaderCanBecomeOwner(t *testing.T) {
	ctx := context.Background()
	a, r := newUser("a"), newUser("r")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		r.ID(): {Role: role.RoleReader},
	})
	f := newTransferFixture(t, ws, a, r)

	_, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: r.ID()})
	require.NoError(t, err)

	saved, err := f.wsRepo.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(r.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(a.ID()))
}

func TestTransferOwner_Rejections(t *testing.T) {
	a, b, ghost := newUser("a"), newUser("b"), newUser("ghost")
	outsider := newUser("outsider")

	tests := []struct {
		name   string
		ws     *workspace.Workspace
		users  []*user.User
		target user.ID
		want   error
	}{
		{
			name:   "personal workspace",
			ws:     workspace.New().NewID().Name("Solo").Alias("solo").Members(map[workspace.UserID]workspace.Member{a.ID(): {Role: role.RoleOwner}}).Personal(true).MustBuild(),
			users:  []*user.User{a},
			target: a.ID(),
			want:   ErrPersonalWorkspace,
		},
		{
			name:   "target not a member",
			ws:     teamWs(map[workspace.UserID]workspace.Member{a.ID(): {Role: role.RoleOwner}}),
			users:  []*user.User{a, outsider},
			target: outsider.ID(),
			want:   ErrTargetNotMember,
		},
		{
			name: "target disabled",
			ws: teamWs(map[workspace.UserID]workspace.Member{
				a.ID(): {Role: role.RoleOwner},
				b.ID(): {Role: role.RoleWriter, Disabled: true},
			}),
			users:  []*user.User{a, b},
			target: b.ID(),
			want:   ErrTargetDisabled,
		},
		{
			name: "target user record missing",
			ws: teamWs(map[workspace.UserID]workspace.Member{
				a.ID():     {Role: role.RoleOwner},
				ghost.ID(): {Role: role.RoleWriter},
			}),
			users:  []*user.User{a},
			target: ghost.ID(),
			want:   ErrTargetUserNotFound,
		},
		{
			name: "target already sole owner",
			ws: teamWs(map[workspace.UserID]workspace.Member{
				a.ID(): {Role: role.RoleOwner},
				b.ID(): {Role: role.RoleMaintainer},
			}),
			users:  []*user.User{a, b},
			target: a.ID(),
			want:   ErrAlreadyOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			f := newTransferFixture(t, tt.ws, tt.users...)
			before := tt.ws.Members().Users()

			_, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: tt.ws.ID(), NewOwnerID: tt.target})
			assert.ErrorIs(t, err, tt.want)
			assert.False(t, errors.Is(err, rerror.ErrNotFound), "must not map to 404")
			assert.False(t, f.tx.IsCommitted())

			saved, err := f.wsRepo.FindByID(ctx, tt.ws.ID())
			require.NoError(t, err)
			assert.Equal(t, before, saved.Members().Users())
			for _, u := range tt.users {
				f.assertNoPermittable(t, u.ID())
			}
		})
	}
}

func TestTransferOwner_WorkspaceNotFound(t *testing.T) {
	a := newUser("a")
	f := newTransferFixture(t, teamWs(map[workspace.UserID]workspace.Member{a.ID(): {Role: role.RoleOwner}}), a)

	_, err := f.uc.Execute(context.Background(), TransferOwnerInput{WorkspaceID: workspace.NewID(), NewOwnerID: a.ID()})
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

type failingSaveWorkspaceRepo struct {
	workspace.Repo
	err error
}

func (r failingSaveWorkspaceRepo) Save(context.Context, *workspace.Workspace) error { return r.err }

func TestTransferOwner_SaveErrorIsNotCommitted(t *testing.T) {
	a, b := newUser("a"), newUser("b")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleWriter},
	})
	f := newTransferFixture(t, ws, a, b)
	saveErr := errors.New("save failed")
	uc := NewTransferOwnerUseCase(failingSaveWorkspaceRepo{Repo: f.wsRepo, err: saveErr}, f.userRepo, f.roleRepo, f.permittableRepo, f.tx)

	_, err := uc.Execute(context.Background(), TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	assert.ErrorIs(t, err, saveErr)
	assert.False(t, f.tx.IsCommitted())
}

func TestTransferOwner_ReusesExistingPermittable(t *testing.T) {
	ctx := context.Background()
	a, b := newUser("a"), newUser("b")
	other := workspace.NewID()
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleWriter},
	})
	f := newTransferFixture(t, ws, a, b)
	existing := permittable.New().NewID().UserID(b.ID()).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(ws.ID(), f.roles[role.RoleWriter].ID()),
			permittable.NewWorkspaceRole(other, f.roles[role.RoleReader].ID()),
		}).MustBuild()
	require.NoError(t, f.permittableRepo.Save(ctx, *existing))

	_, err := f.uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.NoError(t, err)

	f.assertPermittableRole(t, b.ID(), ws.ID(), role.RoleOwner)
	f.assertPermittableRole(t, b.ID(), other, role.RoleReader)
}
