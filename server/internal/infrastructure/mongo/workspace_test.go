package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_FindByID(t *testing.T) {
	ws := workspace.New().NewID().Name("hoge").MustBuild()
	tests := []struct {
		Name               string
		Input              id.WorkspaceID
		RepoData, Expected *workspace.Workspace
		WantErr            bool
	}{
		{
			Name:     "must find a workspace",
			Input:    ws.ID(),
			RepoData: ws,
			Expected: ws,
		},
		{
			Name:     "must not find any workspace",
			Input:    id.NewWorkspaceID(),
			RepoData: ws,
			WantErr:  true,
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.Save(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, err := repo.FindByID(ctx, tc.Input)
			if tc.WantErr {
				assert.Equal(tt, err, rerror.ErrNotFound)
			} else {
				assert.Equal(tt, tc.Expected.ID(), got.ID())
				assert.Equal(tt, tc.Expected.Name(), got.Name())
			}
		})
	}
}

func TestWorkspace_FindByIDs(t *testing.T) {
	ws1 := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()
	ws3 := workspace.New().NewID().Name("xxx").MustBuild()

	tests := []struct {
		Name               string
		Input              id.WorkspaceIDList
		RepoData, Expected workspace.List
	}{
		{
			Name:     "must find users",
			RepoData: workspace.List{ws1, ws2},
			Input:    id.WorkspaceIDList{ws1.ID(), ws2.ID()},
			Expected: workspace.List{ws1, ws2},
		},
		{
			Name:     "must not find any user",
			Input:    id.WorkspaceIDList{ws3.ID()},
			RepoData: workspace.List{ws2, ws1},
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.SaveAll(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, err := repo.FindByIDs(ctx, tc.Input)
			assert.NoError(tt, err)
			for k, ws := range got {
				if ws != nil {
					assert.Equal(tt, tc.Expected[k].ID(), ws.ID())
					assert.Equal(tt, tc.Expected[k].Name(), ws.Name())
				}
			}
		})
	}
}

func TestWorkspace_FindByName(t *testing.T) {
	ws1 := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()
	ws3 := workspace.New().NewID().Name("xxx").MustBuild()

	tests := []struct {
		Name          string
		Input         string
		RepoData      workspace.List
		Expected      *workspace.Workspace
		ExpectedError error
	}{
		{
			Name:          "must find user",
			RepoData:      workspace.List{ws1, ws2, ws3},
			Input:         ws1.Name(),
			Expected:      ws1,
			ExpectedError: nil,
		},
		{
			Name:          "do not find user",
			RepoData:      workspace.List{ws1, ws2, ws3},
			Input:         "notfound",
			Expected:      nil,
			ExpectedError: rerror.ErrNotFoundRaw,
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.SaveAll(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, err := repo.FindByName(ctx, tc.Input)
			if tc.ExpectedError != nil {
				assert.EqualError(tt, err, tc.ExpectedError.Error())
				assert.Equal(tt, got, tc.Expected)
			} else {
				assert.NoError(tt, err)
				assert.Equal(tt, tc.Expected.ID(), got.ID())
				assert.Equal(tt, tc.Expected.Name(), got.Name())
			}
		})
	}
}

func TestWorkspace_FindByAlias(t *testing.T) {
	ws1 := workspace.New().NewID().Name("hoge").Alias("alias").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Alias("alias2").MustBuild()
	ws3 := workspace.New().NewID().Name("xxx").Alias("alias3").MustBuild()

	tests := []struct {
		Name          string
		Input         string
		RepoData      workspace.List
		Expected      *workspace.Workspace
		ExpectedError error
	}{
		{
			Name:          "must find user",
			RepoData:      workspace.List{ws1, ws2, ws3},
			Input:         ws1.Alias(),
			Expected:      ws1,
			ExpectedError: nil,
		},
		{
			Name:          "do not find user",
			RepoData:      workspace.List{ws1, ws2, ws3},
			Input:         "notfound",
			Expected:      nil,
			ExpectedError: rerror.ErrNotFoundRaw,
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.SaveAll(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, err := repo.FindByAlias(ctx, tc.Input)
			if tc.ExpectedError != nil {
				assert.EqualError(tt, err, tc.ExpectedError.Error())
				assert.Equal(tt, got, tc.Expected)
			} else {
				assert.NoError(tt, err)
				assert.NotNil(tt, got)
				assert.Equal(tt, tc.Expected.ID(), got.ID())
				assert.Equal(tt, tc.Expected.Name(), got.Name())
			}
		})
	}
}

func TestWorkspace_FindByUser(t *testing.T) {
	u := user.New().Name("aaa").NewID().Email("aaa@bbb.com").MustBuild()
	ws := workspace.New().NewID().Name("hoge").Members(map[user.ID]workspace.Member{u.ID(): {Role: workspace.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	tests := []struct {
		Name     string
		Input    id.UserID
		RepoData *workspace.Workspace
		Expected workspace.List
	}{
		{
			Name:     "must find a workspace",
			Input:    u.ID(),
			RepoData: ws,
			Expected: workspace.List{ws},
		},
		{
			Name:     "must not find any workspace",
			Input:    user.NewID(),
			RepoData: ws,
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.Save(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, err := repo.FindByUser(ctx, tc.Input)
			assert.NoError(tt, err)
			for k, ws := range got {
				if ws != nil {
					assert.Equal(tt, tc.Expected[k].ID(), ws.ID())
					assert.Equal(tt, tc.Expected[k].Name(), ws.Name())
				}
			}
		})
	}
}

func TestWorkspace_FindByUserWithPagination(t *testing.T) {
	u := user.New().Name("aaa").NewID().Email("test@mail.com").MustBuild()
	ws1 := workspace.New().NewID().Name("hoge").Members(map[user.ID]workspace.Member{u.ID(): {Role: workspace.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Members(map[user.ID]workspace.Member{u.ID(): {Role: workspace.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	ws3 := workspace.New().NewID().Name("xxx").Members(map[user.ID]workspace.Member{u.ID(): {Role: workspace.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	tests := []struct {
		Name     string
		Input    id.UserID
		RepoData workspace.List
		Expected workspace.List
	}{
		{
			Name:     "must find a workspace",
			Input:    u.ID(),
			RepoData: workspace.List{ws1, ws2, ws3},
			Expected: workspace.List{ws1, ws2, ws3},
		},
		{
			Name:     "must not find any workspace",
			Input:    user.NewID(),
			RepoData: workspace.List{ws1, ws2, ws3},
			Expected: workspace.List{},
		},
	}
	init := mongotest.Connect(t)
	for _, tc := range tests {
		tc := tc

		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()

			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.SaveAll(ctx, tc.RepoData)
			assert.NoError(tt, err)

			got, _, err := repo.FindByUserWithPagination(ctx, tc.Input, nil)
			assert.NoError(tt, err)
			for k, ws := range got {
				if ws != nil {
					assert.Equal(tt, tc.Expected[k].ID(), ws.ID())
					assert.Equal(tt, tc.Expected[k].Name(), ws.Name())
				}
			}
		})
	}
}

func TestWorkspace_Remove(t *testing.T) {
	ws := workspace.New().NewID().Name("hoge").MustBuild()

	init := mongotest.Connect(t)
	client := mongox.NewClientWithDatabase(init(t))

	repo := NewWorkspace(client)
	ctx := context.Background()
	err := repo.Save(ctx, ws)
	assert.NoError(t, err)

	err = repo.Remove(ctx, ws.ID())
	assert.NoError(t, err)
}

func TestWorkspace_RemoveAll(t *testing.T) {
	ws1 := workspace.New().NewID().Name("hoge").MustBuild()
	ws2 := workspace.New().NewID().Name("foo").MustBuild()

	init := mongotest.Connect(t)
	client := mongox.NewClientWithDatabase(init(t))

	repo := NewWorkspace(client)
	ctx := context.Background()
	err := repo.SaveAll(ctx, workspace.List{ws1, ws2})
	assert.NoError(t, err)

	err = repo.RemoveAll(ctx, id.WorkspaceIDList{ws1.ID(), ws2.ID()})
	assert.NoError(t, err)
}

func TestWorkspace_FindByIntegrations(t *testing.T) {
	u := user.New().Name("aaa").NewID().Email("aaa@bbb.com").MustBuild()
	i1 := workspace.NewIntegrationID()
	i2 := workspace.NewIntegrationID()
	ws1 := workspace.New().NewID().Name("hoge").Integrations(map[workspace.IntegrationID]workspace.Member{i1: {
		Role:      workspace.RoleOwner,
		InvitedBy: u.ID(),
	}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Integrations(map[workspace.IntegrationID]workspace.Member{i2: {
		Role:      workspace.RoleOwner,
		InvitedBy: u.ID(),
	}}).MustBuild()

	tests := []struct {
		Name    string
		Input   id.IntegrationIDList
		data    workspace.List
		want    workspace.List
		wantErr error
	}{
		{
			Name:    "succes find multiple workspaces",
			Input:   id.IntegrationIDList{i1, i2},
			data:    workspace.List{ws1, ws2},
			want:    workspace.List{ws1, ws2},
			wantErr: nil,
		},
		{
			Name:    "success find a workspace",
			Input:   id.IntegrationIDList{i1},
			data:    workspace.List{ws1, ws2},
			want:    workspace.List{ws1},
			wantErr: nil,
		},
		{
			Name:    "success input no integrations",
			Input:   id.IntegrationIDList{},
			data:    workspace.List{ws1, ws2},
			want:    workspace.List{},
			wantErr: nil,
		},
		{
			Name:    "success find no workspaces",
			Input:   id.IntegrationIDList{workspace.NewIntegrationID()},
			want:    workspace.List{},
			wantErr: nil,
		},
	}

	init := mongotest.Connect(t)

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(tt *testing.T) {
			tt.Parallel()
			client := mongox.NewClientWithDatabase(init(t))

			repo := NewWorkspace(client)
			ctx := context.Background()
			err := repo.SaveAll(ctx, tc.data)
			assert.NoError(tt, err)

			got, err := repo.FindByIntegrations(ctx, tc.Input)
			assert.NoError(tt, err)
			assert.Len(tt, got, len(tc.want))
			for k, ws := range got {
				if ws != nil {
					assert.Equal(tt, tc.want[k].ID(), ws.ID())
					assert.Equal(tt, tc.want[k].Name(), ws.Name())
				}
			}

			err = repo.RemoveAll(ctx, id.WorkspaceIDList{ws1.ID(), ws2.ID()})
			assert.NoError(t, err)
		})
	}
}
