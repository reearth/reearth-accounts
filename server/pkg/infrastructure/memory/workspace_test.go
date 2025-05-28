package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/util"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkspace(t *testing.T) {
	expected := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	got := NewWorkspace()
	assert.Equal(t, expected, got)
}

func TestNewWorkspaceWith(t *testing.T) {
	ws := workspace.New().NewID().Name("hoge").MustBuild()

	got, err := NewWorkspaceWith(ws).FindByID(context.Background(), ws.ID())
	assert.NoError(t, err)
	assert.Equal(t, ws, got)
}

func TestWorkspace_FindByID(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	out, err := r.FindByID(ctx, ws.ID())
	assert.NoError(t, err)
	assert.Equal(t, ws, out)

	out2, err := r.FindByID(ctx, id.WorkspaceID{})
	assert.Nil(t, out2)
	assert.Same(t, rerror.ErrNotFound, err)

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Save(ctx, ws))
}

func TestWorkspace_FindByName(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	out, err := r.FindByName(ctx, ws.Name())
	assert.NoError(t, err)
	assert.Equal(t, ws, out)

	out2, err := r.FindByName(ctx, "notfound")
	assert.Nil(t, out2)
	assert.Same(t, rerror.ErrNotFound, err)

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Save(ctx, ws))
}

func TestWorkspace_FindByIDs(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	r.data.Store(ws2.ID(), ws2)

	ids := id.WorkspaceIDList{ws.ID()}
	wsl := workspace.List{ws}
	out, err := r.FindByIDs(ctx, ids)
	assert.NoError(t, err)
	assert.Equal(t, wsl, out)

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Save(ctx, ws))
}

func TestWorkspace_FindByIntegrations(t *testing.T) {
	ctx := context.Background()
	i1 := workspace.NewIntegrationID()
	i2 := workspace.NewIntegrationID()
	ws := workspace.New().NewID().Name("hoge").Integrations(map[workspace.IntegrationID]workspace.Member{i1: {}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Integrations(map[workspace.IntegrationID]workspace.Member{i2: {}}).MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	r.data.Store(ws2.ID(), ws2)

	type args struct {
		ids workspace.IntegrationIDList
	}
	tests := []struct {
		name    string
		args    args
		want    workspace.List
		wantErr error
	}{
		{
			name:    "Success",
			args:    args{ids: workspace.IntegrationIDList{i2}},
			want:    workspace.List{ws2},
			wantErr: nil,
		},
		{
			name:    "Success with multiple integrations",
			args:    args{ids: workspace.IntegrationIDList{i1, i2}},
			want:    workspace.List{ws, ws2},
			wantErr: nil,
		},
		{
			name:    "Success with empty integrations",
			args:    args{ids: workspace.IntegrationIDList{}},
			want:    nil,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := r.FindByIntegrations(ctx, tc.args.ids)
			assert.Equal(t, tc.wantErr, err)
			assert.Equal(t, tc.want, out)
		})
	}
}

func TestWorkspace_FindByUser(t *testing.T) {
	ctx := context.Background()
	u := user.New().NewID().Name("aaa").Email("aaa@bbb.com").MustBuild()
	ws := workspace.New().NewID().Name("hoge").Members(map[id.UserID]workspace.Member{u.ID(): {Role: workspace.RoleOwner}}).MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	wsl := workspace.List{ws}
	out, err := r.FindByUser(ctx, u.ID())
	assert.NoError(t, err)
	assert.Equal(t, wsl, out)

	out2, err := r.FindByUser(ctx, id.UserID{})
	assert.Same(t, rerror.ErrNotFound, err)
	assert.Nil(t, out2)

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Save(ctx, ws))
}

func TestWorkspace_Save(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()

	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	_ = r.Save(ctx, ws)
	assert.Equal(t, 1, r.data.Len())

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Save(ctx, ws))
}

func TestWorkspace_SaveAll(t *testing.T) {
	ctx := context.Background()
	ws1 := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()

	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	_ = r.SaveAll(ctx, []*workspace.Workspace{ws1, ws2})
	assert.Equal(t, 2, r.data.Len())

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Remove(ctx, ws1.ID()))
}

func TestWorkspace_Remove(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	r.data.Store(ws2.ID(), ws2)

	_ = r.Remove(ctx, ws2.ID())
	assert.Equal(t, 1, r.data.Len())

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.Remove(ctx, ws.ID()))
}

func TestWorkspace_RemoveAll(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()
	r := &Workspace{
		data: &util.SyncMap[id.WorkspaceID, *workspace.Workspace]{},
	}
	r.data.Store(ws.ID(), ws)
	r.data.Store(ws2.ID(), ws2)

	ids := id.WorkspaceIDList{ws.ID(), ws2.ID()}
	_ = r.RemoveAll(ctx, ids)
	assert.Equal(t, 0, r.data.Len())

	wantErr := errors.New("test")
	SetWorkspaceError(r, wantErr)
	assert.Same(t, wantErr, r.RemoveAll(ctx, ids))
}
