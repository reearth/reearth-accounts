package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type leaveAllFixture struct {
	db    *repo.Container
	owner id.UserID
	iid   id.IntegrationID
	other id.IntegrationID
	roles map[id.WorkspaceID]role.RoleType
}

func newLeaveAllFixture(t *testing.T) *leaveAllFixture {
	t.Helper()
	f := &leaveAllFixture{
		db:    memory.New(),
		owner: id.NewUserID(),
		iid:   id.NewIntegrationID(),
		other: id.NewIntegrationID(),
		roles: map[id.WorkspaceID]role.RoleType{},
	}
	for _, r := range []role.RoleType{role.RoleReader, role.RoleWriter, role.RoleMaintainer} {
		ws := workspace.New().NewID().Name("ws-" + string(r)).Alias("ws-" + string(r)).
			Members(map[workspace.UserID]workspace.Member{f.owner: {Role: role.RoleOwner}}).
			Integrations(map[workspace.IntegrationID]workspace.Member{
				f.iid:   {Role: r, InvitedBy: f.owner},
				f.other: {Role: role.RoleWriter, InvitedBy: f.owner},
			}).MustBuild()
		require.NoError(t, f.db.Workspace.Save(context.Background(), ws))
		f.roles[ws.ID()] = r
	}
	onlyOther := workspace.New().NewID().Name("other").Alias("other").
		Members(map[workspace.UserID]workspace.Member{f.owner: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{f.other: {Role: role.RoleOwner, InvitedBy: f.owner}}).
		MustBuild()
	require.NoError(t, f.db.Workspace.Save(context.Background(), onlyOther))
	return f
}

func clientActingFor(iid id.IntegrationID) *workspace.Operator {
	return &workspace.Operator{Integration: &iid, Client: &workspace.Client{ID: "c1", Name: "app"}}
}

func (f *leaveAllFixture) membershipsOf(t *testing.T, iid id.IntegrationID) map[id.WorkspaceID]role.RoleType {
	t.Helper()
	ws, err := f.db.Workspace.FindByIntegration(context.Background(), iid)
	if errors.Is(err, rerror.ErrNotFound) {
		return map[id.WorkspaceID]role.RoleType{}
	}
	got := map[id.WorkspaceID]role.RoleType{}
	for _, w := range ws {
		got[w.ID()] = w.Members().IntegrationRole(iid)
	}
	return got
}

func TestWorkspace_ActingIntegrationMemberships(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: true})

	got, err := uc.ActingIntegrationMemberships(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	roles := map[id.WorkspaceID]role.RoleType{}
	for _, m := range got {
		roles[m.Workspace.ID()] = m.Role
	}
	assert.Equal(t, f.roles, roles)
	for i := 1; i < len(got); i++ {
		assert.Negative(t, got[i-1].Workspace.ID().Compare(got[i].Workspace.ID()), "sorted by workspace ID")
	}

	none, err := uc.ActingIntegrationMemberships(ctx, clientActingFor(id.NewIntegrationID()))
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestWorkspace_IntegrationEndpointsRefuseOtherOperators(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: true})
	uid := id.NewUserID()

	for name, op := range map[string]*workspace.Operator{
		"no operator":                   nil,
		"a user's own token":            {User: &uid},
		"a client acting for a user":    {User: &uid, Client: &workspace.Client{ID: "c1"}},
		"an integration without client": {Integration: &f.iid},
		"an anonymous client":           {Client: &workspace.Client{ID: "c1"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.ActingIntegrationMemberships(ctx, op)
			assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
			_, err = uc.LeaveAllAsIntegration(ctx, op)
			assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
		})
	}
	assert.Len(t, f.membershipsOf(t, f.iid), 3)
}

func TestWorkspace_LeaveAllAsIntegration(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: false})
	otherBefore := f.membershipsOf(t, f.other)

	res, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	assert.ElementsMatch(t, keysOf(f.roles), res.Removed)
	assert.Empty(t, res.Failed)
	assert.Empty(t, f.membershipsOf(t, f.iid))
	assert.Equal(t, otherBefore, f.membershipsOf(t, f.other))

	again, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	assert.Empty(t, again.Removed)
	assert.Empty(t, again.Failed)
}

func TestWorkspace_LeaveAllAsIntegration_SkipsAConcurrentRemoval(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	ids := keysOf(f.roles)
	gone := ids[0]
	f.db.Workspace = &afterFindByIntegration{Repo: f.db.Workspace, hook: func() {
		ws, err := f.db.Workspace.(*afterFindByIntegration).Repo.FindByID(ctx, gone)
		require.NoError(t, err)
		require.NoError(t, ws.Members().DeleteIntegration(f.iid))
		require.NoError(t, f.db.Workspace.(*afterFindByIntegration).Repo.Save(ctx, ws))
	}}
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: true})

	res, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	assert.ElementsMatch(t, ids[1:], res.Removed)
	assert.Empty(t, res.Failed)
	assert.Empty(t, f.membershipsOf(t, f.iid))
}

func TestWorkspace_LeaveAllAsIntegration_ContinuesPastAFailure(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	ids := keysOf(f.roles)
	broken := ids[1]
	inner := f.db.Workspace
	f.db.Workspace = &failingLock{Repo: inner, id: broken}
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: true})

	res, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	assert.Equal(t, id.WorkspaceIDList{broken}, res.Failed)
	assert.ElementsMatch(t, []id.WorkspaceID{ids[0], ids[2]}, res.Removed)
	assert.Equal(t, map[id.WorkspaceID]role.RoleType{broken: f.roles[broken]}, f.membershipsOf(t, f.iid))

	f.db.Workspace = inner
	retry, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	require.NoError(t, err)
	assert.Equal(t, id.WorkspaceIDList{broken}, retry.Removed)
	assert.Empty(t, f.membershipsOf(t, f.iid))
}

func TestWorkspace_LeaveAllAsIntegration_FailsWhenTheReadFails(t *testing.T) {
	ctx := context.Background()
	f := newLeaveAllFixture(t)
	readErr := errors.New("read failed")
	f.db.Workspace = &failingFindByIntegration{Repo: f.db.Workspace, err: readErr}
	uc := NewWorkspace(f.db, nil, &fakeCerbos{allowed: true})

	_, err := uc.LeaveAllAsIntegration(ctx, clientActingFor(f.iid))
	assert.ErrorIs(t, err, readErr)
	_, err = uc.ActingIntegrationMemberships(ctx, clientActingFor(f.iid))
	assert.ErrorIs(t, err, readErr)
}

func keysOf(m map[id.WorkspaceID]role.RoleType) id.WorkspaceIDList {
	ids := make(id.WorkspaceIDList, 0, len(m))
	for k := range m {
		ids = append(ids, k)
	}
	return ids
}

type afterFindByIntegration struct {
	workspace.Repo
	hook func()
}

func (r *afterFindByIntegration) FindByIntegration(ctx context.Context, iid id.IntegrationID) (workspace.List, error) {
	ws, err := r.Repo.FindByIntegration(ctx, iid)
	if r.hook != nil {
		r.hook()
		r.hook = nil
	}
	return ws, err
}

type failingLock struct {
	workspace.Repo
	id id.WorkspaceID
}

func (r *failingLock) FindByIDForUpdate(ctx context.Context, wid id.WorkspaceID) (*workspace.Workspace, error) {
	if wid == r.id {
		return nil, errors.New("lock failed")
	}
	return r.Repo.FindByIDForUpdate(ctx, wid)
}

type failingFindByIntegration struct {
	workspace.Repo
	err error
}

func (r *failingFindByIntegration) FindByIntegration(context.Context, id.IntegrationID) (workspace.List, error) {
	return nil, r.err
}
