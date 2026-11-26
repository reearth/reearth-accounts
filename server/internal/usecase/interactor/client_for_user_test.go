package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
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

type clientForUserFixture struct {
	db                 *repo.Container
	ws                 interfaces.Workspace
	users              interfaces.User
	scim               *Scim
	subject, peer      id.UserID
	newcomer           id.UserID
	team               id.WorkspaceID
	integration, spare id.IntegrationID
}

func newClientForUserFixture(t *testing.T) clientForUserFixture {
	t.Helper()
	ctx := context.Background()
	f := clientForUserFixture{
		db:          memory.New(),
		subject:     id.NewUserID(),
		peer:        id.NewUserID(),
		newcomer:    id.NewUserID(),
		team:        id.NewWorkspaceID(),
		integration: id.NewIntegrationID(),
		spare:       id.NewIntegrationID(),
	}
	for name, uid := range map[string]id.UserID{"subject": f.subject, "peer": f.peer, "newcomer": f.newcomer} {
		u := user.New().ID(uid).Name(name).Email(name + "@example.com").
			Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|" + name}}).Workspace(id.NewWorkspaceID()).MustBuild()
		require.NoError(t, f.db.User.Save(ctx, u))
	}
	team := workspace.New().ID(f.team).Name("team").Alias("team").
		Members(map[workspace.UserID]workspace.Member{
			f.subject: {Role: role.RoleOwner, InvitedBy: f.subject},
			f.peer:    {Role: role.RoleReader, InvitedBy: f.subject},
		}).
		Integrations(map[workspace.IntegrationID]workspace.Member{f.integration: {Role: role.RoleOwner, InvitedBy: f.subject}}).
		MustBuild()
	require.NoError(t, f.db.Workspace.Save(ctx, team))
	allow := &fakeCerbos{allowed: true}
	f.ws = NewWorkspace(f.db, nil, allow)
	f.users = NewUser(f.db, nil, allow, "", "")
	f.scim = NewScim(f.db, allow)
	return f
}

func (f clientForUserFixture) own() *workspace.Operator {
	owning := id.WorkspaceIDList{f.team}
	return &workspace.Operator{
		User:                   &f.subject,
		ReadableWorkspaces:     owning,
		WritableWorkspaces:     owning,
		MaintainableWorkspaces: owning,
		OwningWorkspaces:       owning,
	}
}

func (f clientForUserFixture) clientForUser() *workspace.Operator {
	op := f.own()
	op.Client = &workspace.Client{ID: "svc", Name: "svc"}
	return op
}

func (f clientForUserFixture) clientForIntegration() *workspace.Operator {
	owning := id.WorkspaceIDList{f.team}
	return &workspace.Operator{
		Integration:            &f.integration,
		Client:                 &workspace.Client{ID: "svc", Name: "svc"},
		ReadableWorkspaces:     owning,
		WritableWorkspaces:     owning,
		MaintainableWorkspaces: owning,
		OwningWorkspaces:       owning,
	}
}

type operatorCall struct {
	refusal   error
	needsUser bool
	call      func(context.Context, clientForUserFixture, *workspace.Operator) error
}

func writeCalls() map[string]operatorCall {
	invalid := interfaces.ErrInvalidOperator
	return map[string]operatorCall{
		"Workspace.FindAll": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.FindAll(ctx, interfaces.FindAllWorkspacesParam{}, op)
			return err
		}},
		"Workspace.Create": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.Create(ctx, "new", "new", "", f.subject, false, op)
			return err
		}},
		"Workspace.Update": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.Update(ctx, interfaces.UpdateWorkspaceParam{ID: f.team, Name: lo.ToPtr("renamed")}, op)
			return err
		}},
		"Workspace.AddUserMember": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.AddUserMember(ctx, f.team, map[workspace.UserID]role.RoleType{f.newcomer: role.RoleReader}, op)
			return err
		}},
		"Workspace.AddIntegrationMember": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.AddIntegrationMember(ctx, f.team, f.spare, role.RoleReader, op)
			return err
		}},
		"Workspace.RemoveUserMember": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.RemoveUserMember(ctx, f.team, f.peer, op)
			return err
		}},
		"Workspace.RemoveMultipleUserMembers": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.RemoveMultipleUserMembers(ctx, f.team, workspace.UserIDList{f.peer}, op)
			return err
		}},
		"Workspace.RemoveIntegration": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.RemoveIntegration(ctx, f.team, f.integration, op)
			return err
		}},
		"Workspace.RemoveIntegrations": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.RemoveIntegrations(ctx, f.team, workspace.IntegrationIDList{f.integration}, op)
			return err
		}},
		"Workspace.UpdateUserMember": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.UpdateUserMember(ctx, f.team, f.peer, role.RoleWriter, op)
			return err
		}},
		"Workspace.UpdateUserMemberViaService": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.UpdateUserMemberViaService(ctx, f.team, f.peer, role.RoleWriter, op)
			return err
		}},
		"Workspace.UpdateIntegration": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.UpdateIntegration(ctx, f.team, f.integration, role.RoleWriter, op)
			return err
		}},
		"Workspace.Remove": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			return f.ws.Remove(ctx, f.team, op)
		}},
		"Workspace.Deactivate": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.Deactivate(ctx, f.team, op)
			return err
		}},
		"Workspace.Restore": {invalid, false, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.Restore(ctx, f.team, op)
			return err
		}},
		"Workspace.TransferOwnership": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.ws.TransferOwnership(ctx, f.team, f.peer, op)
			return err
		}},
		"User.FindAll": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.users.FindAll(ctx, interfaces.FindAllUsersParam{Operator: op})
			return err
		}},
		"User.Deactivate": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.users.Deactivate(ctx, f.peer, op)
			return err
		}},
		"User.Restore": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.users.Restore(ctx, f.peer, op)
			return err
		}},
		"User.UpdateUserBySub": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			return f.users.UpdateUserBySub(ctx, "auth0|peer", lo.ToPtr("renamed"), op)
		}},
		"User.SetPlatformRolesBySub": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			return f.users.SetPlatformRolesBySub(ctx, "auth0|peer", nil, op)
		}},
		"Scim.GenerateScimToken": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.scim.GenerateScimToken(ctx, f.team, op)
			return err
		}},
		"Scim.GetScimConfig": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.scim.GetScimConfig(ctx, f.team, op)
			return err
		}},
		"Scim.RevokeScimToken": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			return f.scim.RevokeScimToken(ctx, f.team, op)
		}},
		"Scim.UpdateScimConfig": {invalid, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.scim.UpdateScimConfig(ctx, f.team, true, nil, op)
			return err
		}},
		"Scim.GetWorkspaceScimState": {interfaces.ErrPermissionDenied, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.scim.GetWorkspaceScimState(ctx, f.team, op)
			return err
		}},
		"Scim.SetWorkspaceScimAvailable": {interfaces.ErrPermissionDenied, true, func(ctx context.Context, f clientForUserFixture, op *workspace.Operator) error {
			_, err := f.scim.SetWorkspaceScimAvailable(ctx, f.team, true, op)
			return err
		}},
	}
}

func TestClientForUser_WritesAreRefusedWithoutTheGate(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, c operatorCall, op func(clientForUserFixture) *workspace.Operator) error {
		f := newClientForUserFixture(t)
		return c.call(ctx, f, op(f))
	}
	for name, c := range writeCalls() {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, run(t, c, clientForUserFixture.clientForUser), c.refusal, "a client acting for the subject")
			assert.NotErrorIs(t, run(t, c, clientForUserFixture.own), c.refusal, "the subject's own token")
			err := run(t, c, clientForUserFixture.clientForIntegration)
			if c.needsUser {
				assert.ErrorIs(t, err, c.refusal, "a client acting for an integration")
			} else {
				assert.NotErrorIs(t, err, c.refusal, "a client acting for an integration")
			}
		})
	}
}

func TestScim_MaintainersOwnTokenIsServed(t *testing.T) {
	ctx := context.Background()
	f := newClientForUserFixture(t)
	maintainer, err := f.db.User.FindByID(ctx, f.newcomer)
	require.NoError(t, err)
	team, err := f.db.Workspace.FindByID(ctx, f.team)
	require.NoError(t, err)
	require.NoError(t, team.Members().Join(maintainer, role.RoleMaintainer, f.subject))
	require.NoError(t, f.db.Workspace.Save(ctx, team))
	op := &workspace.Operator{
		User:                   &f.newcomer,
		ReadableWorkspaces:     id.WorkspaceIDList{f.team},
		WritableWorkspaces:     id.WorkspaceIDList{f.team},
		MaintainableWorkspaces: id.WorkspaceIDList{f.team},
	}

	available, err := f.scim.SetWorkspaceScimAvailable(ctx, f.team, true, op)
	require.NoError(t, err)
	assert.True(t, available)
	available, err = f.scim.GetWorkspaceScimState(ctx, f.team, op)
	require.NoError(t, err)
	assert.True(t, available)
	_, err = f.scim.UpdateScimConfig(ctx, f.team, true, nil, op)
	require.NoError(t, err)
	token, err := f.scim.GenerateScimToken(ctx, f.team, op)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	cfg, err := f.scim.GetScimConfig(ctx, f.team, op)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled())
	require.NoError(t, f.scim.RevokeScimToken(ctx, f.team, op))
	cfg, err = f.scim.GetScimConfig(ctx, f.team, op)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled())
}
