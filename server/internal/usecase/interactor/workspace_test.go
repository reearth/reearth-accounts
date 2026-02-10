package interactor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_Create(t *testing.T) {
	ctx := context.Background()

	db := memory.New()
	for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
		_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
	}

	u := user.New().NewID().Name("aaa").Email("aaa@bbb.com").Workspace(id.NewWorkspaceID()).MustBuild()
	_ = db.User.Save(ctx, u)
	workspaceUC := NewWorkspace(db, nil, nil)
	op := &workspace.Operator{User: lo.ToPtr(u.ID())}
	ws, err := workspaceUC.Create(ctx, "alias", "name", "description", u.ID(), op)

	assert.NoError(t, err)
	assert.NotNil(t, ws)

	resultWorkspaces, _ := workspaceUC.Fetch(ctx, []workspace.ID{ws.ID()}, &workspace.Operator{
		ReadableWorkspaces: []workspace.ID{ws.ID()},
	})

	assert.NotNil(t, resultWorkspaces)
	assert.NotEmpty(t, resultWorkspaces)
	assert.Equal(t, resultWorkspaces[0].ID(), ws.ID())
	assert.Equal(t, resultWorkspaces[0].Alias(), "alias")
	assert.Equal(t, resultWorkspaces[0].Name(), "name")
	assert.Equal(t, resultWorkspaces[0].Metadata().Description(), "description")
	assert.Equal(t, workspace.IDList{resultWorkspaces[0].ID()}, op.OwningWorkspaces)

	// mock workspace error
	wantErr := errors.New("test")
	memory.SetWorkspaceError(db.Workspace, wantErr)
	workspace2, err := workspaceUC.Create(ctx, "alias2", "name2", "description2", u.ID(), op)
	assert.Nil(t, workspace2)
	assert.Equal(t, wantErr, err)
}

func TestWorkspace_Update(t *testing.T) {
	ctx := context.Background()

	seedRoles := func(db *repo.Container) {
		for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
			_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
		}
	}

	t.Run("success", func(t *testing.T) {
		t.Run("update all fields", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Original Name").
				Alias("original-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Metadata(workspace.MetadataFrom("original desc", "https://original.com", "", "", "")).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:          wsID,
				Name:        lo.ToPtr("Updated Name"),
				Alias:       lo.ToPtr("updated-alias"),
				Description: lo.ToPtr("updated description"),
				Website:     lo.ToPtr("https://updated.com"),
				PhotoURL:    lo.ToPtr("workspaces/test/photo.webp"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "Updated Name", result.Name())
			assert.Equal(t, "updated-alias", result.Alias())
			assert.Equal(t, "updated description", result.Metadata().Description())
			assert.Equal(t, "https://updated.com", result.Metadata().Website())
			assert.Equal(t, "workspaces/test/photo.webp", result.Metadata().PhotoURL())

			// Verify persistence
			persisted, err := db.Workspace.FindByID(ctx, wsID)
			assert.NoError(t, err)
			assert.Equal(t, "Updated Name", persisted.Name())
			assert.Equal(t, "updated-alias", persisted.Alias())
		})

		t.Run("update name only", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Original Name").
				Alias("original-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("New Name"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "New Name", result.Name())
			assert.Equal(t, "original-alias", result.Alias())
		})

		t.Run("update metadata only", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Test").
				Alias("test-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Metadata(workspace.MetadataFrom("", "", "", "", "")).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:          wsID,
				Description: lo.ToPtr("New description"),
				Website:     lo.ToPtr("https://example.com"),
				PhotoURL:    lo.ToPtr("workspaces/wsid/photo.webp"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "Test", result.Name())
			assert.Equal(t, "test-alias", result.Alias())
			assert.Equal(t, "New description", result.Metadata().Description())
			assert.Equal(t, "https://example.com", result.Metadata().Website())
			assert.Equal(t, "workspaces/wsid/photo.webp", result.Metadata().PhotoURL())
		})

		t.Run("same alias does not trigger duplicate check", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Test").
				Alias("same-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:    wsID,
				Alias: lo.ToPtr("same-alias"),
				Name:  lo.ToPtr("Updated Name"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "Updated Name", result.Name())
			assert.Equal(t, "same-alias", result.Alias())
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Run("invalid operator - nil user", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Test").
				Alias("test-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("New Name"),
			}, &workspace.Operator{
				User: nil,
			})

			assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
			assert.Nil(t, result)
		})

		t.Run("cannot modify personal workspace", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Personal WS").
				Alias("personal-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(true).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("New Name"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.ErrorIs(t, err, workspace.ErrCannotModifyPersonalWorkspace)
			assert.Nil(t, result)
		})

		t.Run("operation denied - not owner", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			otherUserID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Test").
				Alias("test-alias").
				Members(map[user.ID]workspace.Member{
					ownerID:     {Role: role.RoleOwner},
					otherUserID: {Role: role.RoleWriter},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("New Name"),
			}, &workspace.Operator{
				User:               lo.ToPtr(otherUserID),
				WritableWorkspaces: []workspace.ID{wsID},
			})

			assert.ErrorIs(t, err, interfaces.ErrOperationDenied)
			assert.Nil(t, result)
		})

		t.Run("empty name validation error", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			ws := workspace.New().
				ID(wsID).
				Name("Original Name").
				Alias("original-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("   "),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.ErrorIs(t, err, user.ErrInvalidName)
			assert.Nil(t, result)
		})

		t.Run("alias already exists", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()
			wsID2 := id.NewWorkspaceID()

			ws := workspace.New().
				ID(wsID).
				Name("Test").
				Alias("test-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, ws))

			// Create another workspace with the alias we want to use
			other := workspace.New().
				ID(wsID2).
				Name("Other").
				Alias("existing-alias").
				Members(map[user.ID]workspace.Member{
					ownerID: {Role: role.RoleOwner},
				}).
				Personal(false).
				MustBuild()
			assert.NoError(t, db.Workspace.Save(ctx, other))

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:    wsID,
				Alias: lo.ToPtr("existing-alias"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.ErrorIs(t, err, interfaces.ErrWorkspaceAliasAlreadyExists)
			assert.Nil(t, result)
		})

		t.Run("workspace not found", func(t *testing.T) {
			db := memory.New()
			seedRoles(db)

			ownerID := id.NewUserID()
			wsID := id.NewWorkspaceID()

			workspaceUC := NewWorkspace(db, nil, nil)
			result, err := workspaceUC.Update(ctx, interfaces.UpdateWorkspaceParam{
				ID:   wsID,
				Name: lo.ToPtr("New Name"),
			}, &workspace.Operator{
				User:             lo.ToPtr(ownerID),
				OwningWorkspaces: []workspace.ID{wsID},
			})

			assert.ErrorIs(t, err, rerror.ErrNotFound)
			assert.Nil(t, result)
		})
	})
}

func TestWorkspace_Fetch(t *testing.T) {
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).MustBuild()

	u := user.New().NewID().Name("aaa").Email("aaa@bbb.com").Workspace(id1).MustBuild()
	op := &workspace.Operator{
		User:               lo.ToPtr(u.ID()),
		ReadableWorkspaces: []workspace.ID{id1, id2},
	}

	tests := []struct {
		name  string
		seeds workspace.List
		args  struct {
			ids      []workspace.ID
			operator *workspace.Operator
		}
		want             workspace.List
		mockWorkspaceErr bool
		wantErr          error
	}{
		{
			name:  "Fetch 1 of 2",
			seeds: workspace.List{w1, w2},
			args: struct {
				ids      []workspace.ID
				operator *workspace.Operator
			}{
				ids:      []workspace.ID{id1},
				operator: op,
			},
			want:    workspace.List{w1},
			wantErr: nil,
		},
		{
			name:  "Fetch 2 of 2",
			seeds: workspace.List{w1, w2},
			args: struct {
				ids      []workspace.ID
				operator *workspace.Operator
			}{
				ids:      []workspace.ID{id1, id2},
				operator: op,
			},
			want:    workspace.List{w1, w2},
			wantErr: nil,
		},
		{
			name:  "Fetch 1 of 0",
			seeds: workspace.List{},
			args: struct {
				ids      []workspace.ID
				operator *workspace.Operator
			}{
				ids:      []workspace.ID{id1},
				operator: op,
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name:  "Fetch 2 of 0",
			seeds: workspace.List{},
			args: struct {
				ids      []workspace.ID
				operator *workspace.Operator
			}{
				ids:      []workspace.ID{id1, id2},
				operator: op,
			},
			want:    nil,
			wantErr: nil,
		},
		{
			name:             "mock error",
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()

			ctx := context.Background()
			db := memory.New()
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.Fetch(ctx, tc.args.ids, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWorkspace_FindByUser(t *testing.T) {
	userID := id.NewUserID()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Members(map[user.ID]workspace.Member{userID: {Role: role.RoleReader}}).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).MustBuild()

	u := user.New().NewID().Name("aaa").Email("aaa@bbb.com").Workspace(id1).MustBuild()
	op := &workspace.Operator{
		User:               lo.ToPtr(u.ID()),
		ReadableWorkspaces: []workspace.ID{id1, id2},
	}

	tests := []struct {
		name  string
		seeds workspace.List
		args  struct {
			userID   user.ID
			operator *workspace.Operator
		}
		want             workspace.List
		mockWorkspaceErr bool
		wantErr          error
	}{
		{
			name:  "Fetch 1 of 2",
			seeds: workspace.List{w1, w2},
			args: struct {
				userID   user.ID
				operator *workspace.Operator
			}{
				userID:   userID,
				operator: op,
			},
			want:    workspace.List{w1},
			wantErr: nil,
		},
		{
			name:  "Fetch 1 of 0",
			seeds: workspace.List{},
			args: struct {
				userID   user.ID
				operator *workspace.Operator
			}{
				userID:   userID,
				operator: op,
			},
			want:    nil,
			wantErr: rerror.ErrNotFound,
		},
		{
			name:  "Fetch 0 of 1",
			seeds: workspace.List{w2},
			args: struct {
				userID   user.ID
				operator *workspace.Operator
			}{
				userID:   userID,
				operator: op,
			},
			want:    nil,
			wantErr: rerror.ErrNotFound,
		},
		{
			name:             "mock error",
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.FindByUser(ctx, tc.args.userID, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWorkspace_Remove(t *testing.T) {
	userID := id.NewUserID()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").MustBuild()
	id3 := id.NewWorkspaceID()
	w3 := workspace.New().ID(id3).Name("W3").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleReader}}).MustBuild()
	id4 := id.NewWorkspaceID()
	w4 := workspace.New().ID(id4).Name("W4").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(true).MustBuild()
	id5 := id.NewWorkspaceID()
	id6 := id.NewWorkspaceID()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1, id2, id3},
		OwningWorkspaces:   []workspace.ID{id1, id4, id5, id6},
	}

	tests := []struct {
		name  string
		seeds workspace.List
		args  struct {
			wId      workspace.ID
			operator *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             *workspace.Workspace
	}{
		{
			name:  "Remove 1",
			seeds: workspace.List{w1, w2},
			args: struct {
				wId      workspace.ID
				operator *workspace.Operator
			}{
				wId:      id1,
				operator: op,
			},
			wantErr: nil,
			want:    nil,
		},
		{
			name:  "Update 2",
			seeds: workspace.List{w1, w2},
			args: struct {
				wId      workspace.ID
				operator *workspace.Operator
			}{
				wId:      id2,
				operator: op,
			},
			wantErr: interfaces.ErrOperationDenied,
			want:    w2,
		},
		{
			name:  "Update 3",
			seeds: workspace.List{w3},
			args: struct {
				wId      workspace.ID
				operator *workspace.Operator
			}{
				wId:      id3,
				operator: op,
			},
			wantErr: interfaces.ErrOperationDenied,
			want:    w3,
		},
		{
			name:  "Remove 4",
			seeds: workspace.List{w4},
			args: struct {
				wId      workspace.ID
				operator *workspace.Operator
			}{
				wId:      id4,
				operator: op,
			},
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
			want:    w4,
		},
		{
			name: "mock workspace error",
			args: struct {
				wId      workspace.ID
				operator *workspace.Operator
			}{
				wId:      id5,
				operator: op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
				_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
			}
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)
			err := workspaceUC.Remove(ctx, tc.args.wId, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}

			assert.NoError(t, err)
			got, err := db.Workspace.FindByID(ctx, tc.args.wId)
			if tc.want == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWorkspace_AddMember(t *testing.T) {
	userID := id.NewUserID()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id3 := id.NewWorkspaceID()
	w3 := workspace.New().ID(id3).Name("W3").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(true).MustBuild()
	id4 := id.NewWorkspaceID()
	w4 := workspace.New().ID(id3).Name("W4").Members(map[user.ID]workspace.Member{id.NewUserID(): {Role: role.RoleOwner}}).Personal(true).MustBuild()

	u := user.New().NewID().Name("aaa").Email("a@b.c").MustBuild()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1, id2},
		OwningWorkspaces:   []workspace.ID{id1, id2, id3},
	}

	tests := []struct {
		name       string
		seeds      workspace.List
		usersSeeds []*user.User
		enforcer   WorkspaceMemberCountEnforcer
		args       struct {
			wId      workspace.ID
			users    map[user.ID]role.RoleType
			operator *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             *workspace.Members
	}{
		{
			name:       "add a member",
			seeds:      workspace.List{w2},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: w2.ID(),
				users: map[user.ID]role.RoleType{
					u.ID(): role.RoleReader,
				},
				operator: op,
			},
			wantErr: nil,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID: {Role: role.RoleOwner},
				u.ID(): {Role: role.RoleReader, InvitedBy: userID}, // added
			}, nil, false),
		},
		{
			name:       "add a non existing member",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: w1.ID(),
				users: map[user.ID]role.RoleType{
					id.NewUserID(): role.RoleReader,
				},
				operator: op,
			},
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID: {Role: role.RoleOwner},
			}, nil, false),
		},
		{
			name:       "add a mamber to personal workspace",
			seeds:      workspace.List{w3},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: w3.ID(),
				users: map[user.ID]role.RoleType{
					u.ID(): role.RoleReader,
				},
				operator: op,
			},
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID: {Role: role.RoleOwner},
			}, map[id.IntegrationID]workspace.Member{}, true),
		},
		{
			name:       "add member but enforcer rejects",
			seeds:      workspace.List{w2},
			usersSeeds: []*user.User{u},
			enforcer: func(_ context.Context, _ *workspace.Workspace, _ user.List, _ *workspace.Operator) error {
				return errors.New("test")
			},
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: w2.ID(),
				users: map[user.ID]role.RoleType{
					u.ID(): role.RoleReader,
				},
				operator: op,
			},
			wantErr: errors.New("test"),
		},
		{
			name:  "op denied",
			seeds: workspace.List{w4},
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: id4,
				users: map[user.ID]role.RoleType{
					id.NewUserID(): role.RoleReader,
				},
				operator: op,
			},
			wantErr:          interfaces.ErrOperationDenied,
			mockWorkspaceErr: false,
		},
		{
			name: "mock error",
			args: struct {
				wId      workspace.ID
				users    map[user.ID]role.RoleType
				operator *workspace.Operator
			}{
				wId: id3,
				users: map[user.ID]role.RoleType{
					u.ID(): role.RoleReader,
				},
				operator: op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
				_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
			}
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.usersSeeds {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, tc.enforcer, nil)

			got, err := workspaceUC.AddUserMember(ctx, tc.args.wId, tc.args.users, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Members())

			got, err = db.Workspace.FindByID(ctx, tc.args.wId)
			if tc.want == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got.Members())
		})
	}
}

func TestWorkspace_AddIntegrationMember(t *testing.T) {
	userID := id.NewUserID()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id2 := id.NewWorkspaceID()
	id3 := id.NewWorkspaceID()
	u := user.New().NewID().Name("aaa").Email("a@b.c").MustBuild()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1, id2},
		OwningWorkspaces:   []workspace.ID{id1, id2, id3},
	}

	iid1 := id.NewIntegrationID()

	tests := []struct {
		name       string
		seeds      workspace.List
		usersSeeds []*user.User
		args       struct {
			wId           workspace.ID
			integrationID id.IntegrationID
			role          role.RoleType
			operator      *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             []id.IntegrationID
	}{
		{
			name:       "add non existing",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u},
			args: struct {
				wId           workspace.ID
				integrationID id.IntegrationID
				role          role.RoleType
				operator      *workspace.Operator
			}{
				wId:           id1,
				integrationID: iid1,
				role:          role.RoleReader,
				operator:      op,
			},
			want: []id.IntegrationID{iid1},
		},
		{
			name: "mock error",
			args: struct {
				wId           workspace.ID
				integrationID id.IntegrationID
				role          role.RoleType
				operator      *workspace.Operator
			}{
				wId:           id1,
				integrationID: iid1,
				role:          role.RoleReader,
				operator:      op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.usersSeeds {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}

			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.AddIntegrationMember(ctx, tc.args.wId, tc.args.integrationID, tc.args.role, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Members().IntegrationIDs())
		})
	}
}

func TestWorkspace_RemoveMember(t *testing.T) {
	userID := id.NewUserID()
	u := user.New().NewID().Name("aaa").Email("a@b.c").MustBuild()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}, u.ID(): {Role: role.RoleReader}}).Personal(false).MustBuild()
	id3 := id.NewWorkspaceID()
	w3 := workspace.New().ID(id3).Name("W3").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(true).MustBuild()
	id4 := id.NewWorkspaceID()
	w4 := workspace.New().ID(id4).Name("W4").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()

	op := &workspace.Operator{
		User:                   &userID,
		ReadableWorkspaces:     []workspace.ID{id1, id2},
		OwningWorkspaces:       []workspace.ID{id1},
		MaintainableWorkspaces: []workspace.ID{id2, id3, id4},
	}

	tests := []struct {
		name       string
		seeds      workspace.List
		usersSeeds []*user.User
		args       struct {
			wId      workspace.ID
			uId      user.ID
			operator *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             *workspace.Members
	}{
		{
			name:       "Remove non existing",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				operator *workspace.Operator
			}{
				wId:      id1,
				uId:      id.NewUserID(),
				operator: op,
			},
			wantErr: workspace.ErrTargetUserNotInTheWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, false),
		},
		{
			name:       "Remove",
			seeds:      workspace.List{w2},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				operator *workspace.Operator
			}{
				wId:      id2,
				uId:      u.ID(),
				operator: op,
			},
			wantErr: nil,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, nil, false),
		},
		{
			name:       "Remove personal workspace",
			seeds:      workspace.List{w3},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				operator *workspace.Operator
			}{
				wId:      id3,
				uId:      userID,
				operator: op,
			},
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, false),
		},
		{
			name:       "Remove single member",
			seeds:      workspace.List{w4},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				operator *workspace.Operator
			}{
				wId:      id4,
				uId:      userID,
				operator: op,
			},
			wantErr: interfaces.ErrOwnerCannotLeaveTheWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, false),
		},
		{
			name: "mock error",
			args: struct {
				wId      workspace.ID
				uId      user.ID
				operator *workspace.Operator
			}{
				wId:      id1,
				uId:      userID,
				operator: op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
				_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
			}
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.usersSeeds {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.RemoveUserMember(ctx, tc.args.wId, tc.args.uId, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Members())

			got, err = db.Workspace.FindByID(ctx, tc.args.wId)
			if tc.want == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got.Members())
		})
	}
}

func TestWorkspace_RemoveMultipleMembers(t *testing.T) {
	userID := id.NewUserID()
	userID2 := id.NewUserID()
	userID3 := id.NewUserID()
	userID4 := id.NewUserID()
	u := user.New().ID(userID).Name("aaa").Email("a@b.c").MustBuild()
	u2 := user.New().ID(userID2).Name("bbb").Email("b@b.c").MustBuild()
	u3 := user.New().ID(userID3).Name("ccc").Email("c@b.c").MustBuild()
	u4 := user.New().ID(userID4).Name("ddd").Email("d@b.c").MustBuild()

	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").
		Members(map[user.ID]workspace.Member{
			userID:  {Role: role.RoleOwner},
			userID2: {Role: role.RoleReader},
			userID3: {Role: role.RoleReader},
			userID4: {Role: role.RoleReader},
		}).Personal(false).MustBuild()

	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").
		Members(map[user.ID]workspace.Member{
			userID:  {Role: role.RoleOwner},
			userID2: {Role: role.RoleReader},
		}).Personal(true).MustBuild()

	id3 := id.NewWorkspaceID()
	w3 := workspace.New().ID(id3).Name("W3").
		Members(map[user.ID]workspace.Member{
			userID:  {Role: role.RoleOwner},
			userID2: {Role: role.RoleReader},
		}).Personal(false).MustBuild()

	id4 := id.NewWorkspaceID()
	w4 := workspace.New().ID(id4).Name("W4").
		Members(map[user.ID]workspace.Member{
			userID:  {Role: role.RoleOwner},
			userID2: {Role: role.RoleReader},
		}).Personal(false).MustBuild()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1},
		OwningWorkspaces:   []workspace.ID{id1, id2, id3, id4},
	}

	tests := []struct {
		name       string
		seeds      workspace.List
		usersSeeds []*user.User
		args       struct {
			wId      workspace.ID
			uIds     workspace.UserIDList
			operator *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             *workspace.Members
	}{
		{
			name:       "Remove non existing",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id1,
				uIds:     workspace.UserIDList{id.NewUserID()},
				operator: op,
			},
			wantErr: workspace.ErrTargetUserNotInTheWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, false),
		},
		{
			name:       "Remove multiple existing members",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u, u2, u3, u4},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id1,
				uIds:     workspace.UserIDList{userID2, userID3},
				operator: op,
			},
			wantErr: nil,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID4: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name:       "Invalid Operator",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u, u2, u3, u4},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id1,
				uIds:     workspace.UserIDList{userID2, userID3},
				operator: &workspace.Operator{},
			},
			wantErr: interfaces.ErrInvalidOperator,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID4: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name:       "Operation Denied",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u, u2, u3, u4},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:  id1,
				uIds: workspace.UserIDList{userID2},
				operator: &workspace.Operator{
					User:               &userID3,
					ReadableWorkspaces: []workspace.ID{id1},
				},
			},
			wantErr: interfaces.ErrOperationDenied,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID3: {Role: role.RoleReader},
				userID4: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name:       "Remove multiple members, cannot remove from personal workspace",
			seeds:      workspace.List{w2},
			usersSeeds: []*user.User{u, u2},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id2,
				uIds:     workspace.UserIDList{userID, userID2},
				operator: op,
			},
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID2: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name:       "Remove multiple members, cannot remove owner",
			seeds:      workspace.List{w3},
			usersSeeds: []*user.User{u, u2},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id3,
				uIds:     workspace.UserIDList{userID, userID2},
				operator: op,
			},
			wantErr: interfaces.ErrOwnerCannotLeaveTheWorkspace,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID2: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name:       "Remove multiple members, empty user id list",
			seeds:      workspace.List{w4},
			usersSeeds: []*user.User{u, u2},
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id4,
				uIds:     workspace.UserIDList{},
				operator: op,
			},
			wantErr: workspace.ErrNoSpecifiedUsers,
			want: workspace.NewMembersWith(map[user.ID]workspace.Member{
				userID:  {Role: role.RoleOwner},
				userID2: {Role: role.RoleReader},
			}, nil, false),
		},
		{
			name: "mock error",
			args: struct {
				wId      workspace.ID
				uIds     workspace.UserIDList
				operator *workspace.Operator
			}{
				wId:      id1,
				uIds:     workspace.UserIDList{userID2, userID3},
				operator: op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
				_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
			}
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.usersSeeds {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.RemoveMultipleUserMembers(ctx, tc.args.wId, tc.args.uIds, tc.args.operator)
			if tc.wantErr != nil {
				assert.ErrorIs(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Members())

			got, err = db.Workspace.FindByID(ctx, tc.args.wId)
			if tc.want == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got.Members())
		})
	}
}

func TestWorkspace_UpdateMember(t *testing.T) {
	userID := id.NewUserID()
	u := user.New().NewID().Name("aaa").Email("a@b.c").MustBuild()
	id1 := id.NewWorkspaceID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(false).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}, u.ID(): {Role: role.RoleReader}}).Personal(false).MustBuild()
	id3 := id.NewWorkspaceID()
	w3 := workspace.New().ID(id3).Name("W3").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Personal(true).MustBuild()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1, id2},
		OwningWorkspaces:   []workspace.ID{id1, id2, id3},
	}

	tests := []struct {
		name       string
		seeds      workspace.List
		usersSeeds []*user.User
		args       struct {
			wId      workspace.ID
			uId      user.ID
			role     role.RoleType
			operator *workspace.Operator
		}
		wantErr          error
		mockWorkspaceErr bool
		want             *workspace.Members
	}{
		{
			name:       "Update non existing",
			seeds:      workspace.List{w1},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				role     role.RoleType
				operator *workspace.Operator
			}{
				wId:      id1,
				uId:      id.NewUserID(),
				role:     role.RoleWriter,
				operator: op,
			},
			wantErr: workspace.ErrTargetUserNotInTheWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, false),
		},
		{
			name:       "Update",
			seeds:      workspace.List{w2},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				role     role.RoleType
				operator *workspace.Operator
			}{
				wId:      id2,
				uId:      u.ID(),
				role:     role.RoleWriter,
				operator: op,
			},
			wantErr: nil,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}, u.ID(): {Role: role.RoleWriter}}, nil, false),
		},
		{
			name:       "Update personal workspace",
			seeds:      workspace.List{w3},
			usersSeeds: []*user.User{u},
			args: struct {
				wId      workspace.ID
				uId      user.ID
				role     role.RoleType
				operator *workspace.Operator
			}{
				wId:      id3,
				uId:      userID,
				role:     role.RoleReader,
				operator: op,
			},
			wantErr: workspace.ErrCannotModifyPersonalWorkspace,
			want:    workspace.NewMembersWith(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}, map[id.IntegrationID]workspace.Member{}, true),
		},
		{
			name: "mock error",
			args: struct {
				wId      workspace.ID
				uId      user.ID
				role     role.RoleType
				operator *workspace.Operator
			}{
				wId:      id3,
				operator: op,
			},
			wantErr:          errors.New("test"),
			mockWorkspaceErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
				_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
			}
			if tc.mockWorkspaceErr {
				memory.SetWorkspaceError(db.Workspace, tc.wantErr)
			}
			for _, p := range tc.seeds {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.usersSeeds {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}
			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.UpdateUserMember(ctx, tc.args.wId, tc.args.uId, tc.args.role, tc.args.operator)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Members())

			got, err = db.Workspace.FindByID(ctx, tc.args.wId)
			if tc.want == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got.Members())
		})
	}
}

func TestWorkspace_RemoveIntegrations(t *testing.T) {
	userID := id.NewUserID()
	id1 := id.NewWorkspaceID()
	iid1 := id.NewIntegrationID()
	iid2 := id.NewIntegrationID()
	iid3 := id.NewIntegrationID()
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{
			iid1: {Role: role.RoleOwner},
		}).MustBuild()
	id2 := id.NewWorkspaceID()
	w2 := workspace.New().ID(id2).Name("W2").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{
			iid1: {Role: role.RoleReader},
			iid2: {Role: role.RoleMaintainer},
		}).MustBuild()
	w3 := workspace.New().ID(id2).Name("W3").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).
		Integrations(map[workspace.IntegrationID]workspace.Member{
			iid1: {Role: role.RoleReader},
			iid2: {Role: role.RoleMaintainer},
		}).MustBuild()
	id3 := id.NewWorkspaceID()
	u := user.New().NewID().Name("aaa").Email("a@b.c").MustBuild()

	op := &workspace.Operator{
		User:               &userID,
		ReadableWorkspaces: []workspace.ID{id1, id2},
		OwningWorkspaces:   []workspace.ID{id1, id2, id3},
	}

	opEmpty := &workspace.Operator{}

	type args struct {
		ctx  context.Context
		wId  workspace.ID
		iIds workspace.IntegrationIDList
		op   *workspace.Operator
	}

	type seeds struct {
		wList []*workspace.Workspace
		uList []*user.User
	}

	tests := []struct {
		name    string
		args    args
		seeds   seeds
		want    *workspace.Workspace
		wantErr error
	}{
		{
			name: "Remove integration from workspace",
			args: args{
				ctx:  context.Background(),
				wId:  id1,
				iIds: workspace.IntegrationIDList{iid1},
				op:   op,
			},
			seeds: seeds{
				wList: []*workspace.Workspace{w1},
				uList: []*user.User{u},
			},
			want:    workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).Integrations(map[workspace.IntegrationID]workspace.Member{}).MustBuild(),
			wantErr: nil,
		},
		{
			name: "Remove multiple integrations from workspace",
			args: args{
				ctx:  context.Background(),
				wId:  id2,
				iIds: workspace.IntegrationIDList{iid1, iid2},
				op:   op,
			},
			seeds: seeds{
				wList: []*workspace.Workspace{w2},
				uList: []*user.User{u},
			},
			want: workspace.New().ID(id2).Name("W2").Members(map[user.ID]workspace.Member{userID: {Role: role.RoleOwner}}).
				Integrations(map[workspace.IntegrationID]workspace.Member{}).MustBuild(),
			wantErr: nil,
		},
		{
			name: "Partial remove integrations from workspace not allowed",
			args: args{
				ctx:  context.Background(),
				wId:  id2,
				iIds: workspace.IntegrationIDList{iid1, iid2, iid3},
				op:   op,
			},
			seeds: seeds{
				wList: []*workspace.Workspace{w3},
				uList: []*user.User{u},
			},
			want:    w3,
			wantErr: fmt.Errorf("%w: %v", workspace.ErrTargetUserNotInTheWorkspace, []workspace.IntegrationID{iid3}),
		},
		{
			name: "invalid operator",
			args: args{
				ctx:  context.Background(),
				wId:  id1,
				iIds: workspace.IntegrationIDList{iid1},
				op:   opEmpty,
			},
			want:    nil,
			wantErr: interfaces.ErrInvalidOperator,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := memory.New()
			for _, p := range tc.seeds.wList {
				err := db.Workspace.Save(ctx, p)
				assert.NoError(t, err)
			}
			for _, p := range tc.seeds.uList {
				err := db.User.Save(ctx, p)
				assert.NoError(t, err)
			}

			workspaceUC := NewWorkspace(db, nil, nil)

			got, err := workspaceUC.RemoveIntegrations(ctx, tc.args.wId, tc.args.iIds, tc.args.op)
			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tc.want.ID(), got.ID())
			assert.Equal(t, tc.want.Name(), got.Name())
			assert.Equal(t, tc.want.Members(), got.Members())
			assert.NotZero(t, got.UpdatedAt(), "updatedAt should be set")
		})
	}
}
