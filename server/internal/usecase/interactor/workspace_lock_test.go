package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/rbac"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockRecorder records which workspaces were read with FindByIDForUpdate.
type lockRecorder struct {
	workspace.Repo
	locked map[workspace.ID]bool
}

func (r *lockRecorder) FindByIDForUpdate(ctx context.Context, wid workspace.ID) (*workspace.Workspace, error) {
	r.locked[wid] = true
	return r.Repo.FindByIDForUpdate(ctx, wid)
}

// Postgres saves a workspace by rewriting all its member rows, so a writer that
// reads without the lock can save a stale copy over a locked writer's change.
func TestWorkspaceWriters_LockTheWorkspace(t *testing.T) {
	ctx := context.Background()
	manage := actionCerbos{rbac.ActionManage: true}

	type env struct {
		ws       interfaces.Workspace
		scim     *Scim
		user     interfaces.User
		team     workspace.ID
		personal workspace.ID
		owner    user.ID
		member   user.ID
		outsider user.ID
		integ    workspace.IntegrationID
		op       *workspace.Operator
	}
	cases := []struct {
		name string
		// lockOf names the workspace the writer must lock; the team workspace when nil.
		lockOf func(e env) workspace.ID
		call   func(e env) error
	}{
		{name: "Update", call: func(e env) error {
			_, err := e.ws.Update(ctx, interfaces.UpdateWorkspaceParam{ID: e.team, Name: lo.ToPtr("renamed")}, e.op)
			return err
		}},
		{name: "AddUserMember", call: func(e env) error {
			_, err := e.ws.AddUserMember(ctx, e.team, map[user.ID]role.RoleType{e.outsider: role.RoleWriter}, e.op)
			return err
		}},
		{name: "AddIntegrationMember", call: func(e env) error {
			_, err := e.ws.AddIntegrationMember(ctx, e.team, id.NewIntegrationID(), role.RoleReader, e.op)
			return err
		}},
		{name: "UpdateIntegration", call: func(e env) error {
			_, err := e.ws.UpdateIntegration(ctx, e.team, e.integ, role.RoleWriter, e.op)
			return err
		}},
		{name: "RemoveIntegration", call: func(e env) error {
			_, err := e.ws.RemoveIntegration(ctx, e.team, e.integ, e.op)
			return err
		}},
		{name: "RemoveIntegrations", call: func(e env) error {
			_, err := e.ws.RemoveIntegrations(ctx, e.team, workspace.IntegrationIDList{e.integ}, e.op)
			return err
		}},
		{name: "UpdateUserMember", call: func(e env) error {
			_, err := e.ws.UpdateUserMember(ctx, e.team, e.member, role.RoleMaintainer, e.op)
			return err
		}},
		{name: "UpdateUserMemberViaService", call: func(e env) error {
			_, err := e.ws.UpdateUserMemberViaService(ctx, e.team, e.member, role.RoleMaintainer, e.op)
			return err
		}},
		{name: "RemoveUserMember", call: func(e env) error {
			_, err := e.ws.RemoveUserMember(ctx, e.team, e.member, e.op)
			return err
		}},
		{name: "TransferOwnership", call: func(e env) error {
			_, err := e.ws.TransferOwnership(ctx, e.team, e.member, e.op)
			return err
		}},
		{name: "Remove", call: func(e env) error {
			return e.ws.Remove(ctx, e.team, e.op)
		}},
		{name: "Deactivate", call: func(e env) error {
			_, err := e.ws.Deactivate(ctx, e.team, e.op)
			return err
		}},
		{name: "Restore", call: func(e env) error {
			_, err := e.ws.Restore(ctx, e.team, e.op)
			return err
		}},
		{name: "GenerateScimToken", call: func(e env) error {
			_, err := e.scim.GenerateScimToken(ctx, e.team, e.op)
			return err
		}},
		{name: "RevokeScimToken", call: func(e env) error {
			return e.scim.RevokeScimToken(ctx, e.team, e.op)
		}},
		{name: "UpdateScimConfig", call: func(e env) error {
			_, err := e.scim.UpdateScimConfig(ctx, e.team, true, nil, e.op)
			return err
		}},
		{name: "SetWorkspaceScimAvailable", call: func(e env) error {
			_, err := e.scim.SetWorkspaceScimAvailable(ctx, e.team, true, e.op)
			return err
		}},
		{name: "ProvisionScimUser", call: func(e env) error {
			_, err := e.scim.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{WorkspaceID: e.team, Email: "new@example.com", ExternalID: "ext-new", Name: "new"})
			return err
		}},
		{name: "DeprovisionScimUser", call: func(e env) error {
			return e.scim.DeprovisionScimUser(ctx, e.team, "ext-member")
		}},
		{name: "DeprovisionScimUserByUserID", call: func(e env) error {
			return e.scim.DeprovisionScimUserByUserID(ctx, e.team, e.member)
		}},
		{name: "ReactivateScimUserByUserID", call: func(e env) error {
			return e.scim.ReactivateScimUserByUserID(ctx, e.team, e.member)
		}},
		{name: "SyncScimGroup", call: func(e env) error {
			return e.scim.SyncScimGroup(ctx, e.team, "g", "g", nil)
		}},
		{name: "DeleteScimGroup", call: func(e env) error {
			return e.scim.DeleteScimGroup(ctx, e.team, "g")
		}},
		{name: "DeleteMe", call: func(e env) error {
			return e.user.DeleteMe(ctx, e.owner, e.op)
		}},
		{
			name:   "UpdateMe",
			lockOf: func(e env) workspace.ID { return e.personal },
			call: func(e env) error {
				_, err := e.user.UpdateMe(ctx, interfaces.UpdateMeParam{Name: lo.ToPtr("renamed")}, e.op)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := memory.New()
			for _, r := range []string{"self", "owner", "maintainer", "writer", "reader"} {
				require.NoError(t, db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild()))
			}
			personal := id.NewWorkspaceID()
			owner := user.New().NewID().Name("owner").Email("owner@example.com").Workspace(personal).MustBuild()
			member := user.New().NewID().Name("member").Email("member@example.com").MustBuild()
			require.NoError(t, db.User.Save(ctx, owner))
			require.NoError(t, db.User.Save(ctx, member))
			outsider := user.New().NewID().Name("outsider").Email("outsider@example.com").MustBuild()
			require.NoError(t, db.User.Save(ctx, outsider))
			require.NoError(t, db.Workspace.Save(ctx, workspace.New().ID(personal).Name("owner").Personal(true).
				Members(map[user.ID]workspace.Member{owner.ID(): {Role: role.RoleOwner}}).MustBuild()))

			scimCfg := workspace.NewScimConfig()
			scimCfg.SetEnabled(true)
			integ := id.NewIntegrationID()
			team := workspace.New().NewID().Name("team").Alias("team").
				Members(map[user.ID]workspace.Member{
					owner.ID():  {Role: role.RoleOwner},
					member.ID(): {Role: role.RoleWriter, ExternalID: "ext-member"},
				}).
				Integrations(map[workspace.IntegrationID]workspace.Member{integ: {Role: role.RoleReader}}).
				ScimConfig(scimCfg).
				MustBuild()
			require.NoError(t, db.Workspace.Save(ctx, team))

			rec := &lockRecorder{Repo: db.Workspace, locked: map[workspace.ID]bool{}}
			db.Workspace = rec
			e := env{
				ws:       NewWorkspace(db, nil, nil),
				scim:     NewScim(db, manage),
				user:     NewUser(db, nil, nil, "", ""),
				team:     team.ID(),
				owner:    owner.ID(),
				personal: personal,
				member:   member.ID(),
				outsider: outsider.ID(),
				integ:    integ,
				op: &workspace.Operator{
					User:                   lo.ToPtr(owner.ID()),
					OwningWorkspaces:       []workspace.ID{team.ID(), personal},
					MaintainableWorkspaces: []workspace.ID{team.ID(), personal},
				},
			}
			want := e.team
			if tc.lockOf != nil {
				want = tc.lockOf(e)
			}

			require.NoError(t, tc.call(e))
			assert.True(t, rec.locked[want], "%s must read the workspace with FindByIDForUpdate", tc.name)
		})
	}
}
