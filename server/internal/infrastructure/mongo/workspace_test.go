package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
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
	wsMixed := workspace.New().NewID().Name("mixed").Alias("MixedCase").MustBuild()

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
		{
			Name:          "must find workspace by alias case-insensitively (lowercase query)",
			RepoData:      workspace.List{wsMixed},
			Input:         "mixedcase",
			Expected:      wsMixed,
			ExpectedError: nil,
		},
		{
			Name:          "must find workspace by alias case-insensitively (uppercase query)",
			RepoData:      workspace.List{wsMixed},
			Input:         "MIXEDCASE",
			Expected:      wsMixed,
			ExpectedError: nil,
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
	ws := workspace.New().NewID().Name("hoge").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
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
	ws1 := workspace.New().NewID().Name("hoge").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	ws3 := workspace.New().NewID().Name("xxx").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
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

// TestWorkspace_FindByUser_WithWildcardIndex applies the same wildcard index
// the AddWorkspaceMembersWildcardIndex migration creates on workspace.members
// (see internal/infrastructure/mongo/migration) and verifies FindByUser /
// FindByUserWithPagination still return the correct workspaces once it's in
// place, so the SCA-01 fix doesn't just create an index but actually keeps
// the by-user lookup functionally correct.
func TestWorkspace_FindByUser_WithWildcardIndex(t *testing.T) {
	u := user.New().Name("aaa").NewID().Email("aaa@bbb.com").MustBuild()
	other := user.New().Name("bbb").NewID().Email("bbb@ccc.com").MustBuild()
	ws1 := workspace.New().NewID().Name("hoge").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Members(map[user.ID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).MustBuild()

	init := mongotest.Connect(t)
	db := init(t)
	client := mongox.NewClientWithDatabase(db)

	indexModel := mongodriver.IndexModel{Keys: bson.D{{Key: "members.$**", Value: 1}}}
	_, err := db.Collection("workspace").Indexes().CreateOne(context.Background(), indexModel)
	assert.NoError(t, err)

	repo := NewWorkspace(client)
	ctx := context.Background()
	assert.NoError(t, repo.SaveAll(ctx, workspace.List{ws1, ws2}))

	got, err := repo.FindByUser(ctx, u.ID())
	assert.NoError(t, err)
	assert.ElementsMatch(t, workspace.List{ws1, ws2}.IDs(), got.IDs())

	got, err = repo.FindByUser(ctx, other.ID())
	assert.NoError(t, err)
	assert.Empty(t, got)

	gotPage, _, err := repo.FindByUserWithPagination(ctx, u.ID(), nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, workspace.List{ws1, ws2}.IDs(), gotPage.IDs())
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
		Role:      role.RoleOwner,
		InvitedBy: u.ID(),
	}}).MustBuild()
	ws2 := workspace.New().NewID().Name("foo").Integrations(map[workspace.IntegrationID]workspace.Member{i2: {
		Role:      role.RoleOwner,
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

func TestWorkspace_FindByAlias_CaseInsensitive(t *testing.T) {
	db := Connect(t)(t)
	client := mongox.NewClientWithDatabase(db)
	repo := NewWorkspace(client)
	ctx := context.Background()

	ws := workspace.New().NewID().Name("casei").Alias("MixedAlias").MustBuild()
	assert.NoError(t, repo.SaveAll(ctx, workspace.List{ws}))

	t.Run("lowercase query finds mixed-case stored alias", func(t *testing.T) {
		got, err := repo.FindByAlias(ctx, "mixedalias")
		assert.NoError(t, err)
		assert.Equal(t, ws.ID(), got.ID())
	})

	t.Run("uppercase query finds mixed-case stored alias", func(t *testing.T) {
		got, err := repo.FindByAlias(ctx, "MIXEDALIAS")
		assert.NoError(t, err)
		assert.Equal(t, ws.ID(), got.ID())
	})

	t.Run("exact match still works", func(t *testing.T) {
		got, err := repo.FindByAlias(ctx, "MixedAlias")
		assert.NoError(t, err)
		assert.Equal(t, ws.ID(), got.ID())
	})
}
