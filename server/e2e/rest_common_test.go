package e2e

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
)

// REST-specific fixed IDs (independent of the GraphQL test fixtures).
var (
	restDemoUID  = id.NewUserID()
	restDemoWID  = id.NewWorkspaceID()
	restOtherUID = id.NewUserID()
	restOtherWID = id.NewWorkspaceID()
)

// seedDemoUser seeds the fixed mock user (app.FIXED_MOCK_USERNAME) with a personal
// workspace so mock-auth REST requests resolve, plus a second user usable as a
// workspace-membership target.
func seedDemoUser(ctx context.Context, r *repo.Container) error {
	// Seed the named roles that updatePermittable looks up by name (it only
	// auto-creates them when REEARTH_MOCK_AUTH=true is set in the process env).
	for _, rt := range []role.RoleType{role.RoleOwner, role.RoleMaintainer, role.RoleWriter, role.RoleReader, role.RoleSelf} {
		rl := role.New().NewID().Name(string(rt)).MustBuild()
		if err := r.Role.Save(ctx, *rl); err != nil {
			return err
		}
	}

	md := user.NewMetadata()
	md.LangFrom("en")
	md.SetTheme(user.ThemeLight)

	demo := user.New().ID(restDemoUID).
		Name(app.FIXED_MOCK_USERNAME).
		Alias("demo-user").
		Email("demo@example.com").
		Metadata(md).
		Workspace(restDemoWID).
		MustBuild()
	if err := r.User.Save(ctx, demo); err != nil {
		return err
	}

	demoWS := workspace.New().ID(restDemoWID).
		Name("Demo Workspace").
		Alias("demo-personal").
		Members(map[idx.ID[id.User]]workspace.Member{
			restDemoUID: {Role: role.RoleOwner, InvitedBy: restDemoUID},
		}).
		Personal(true).
		MustBuild()
	if err := r.Workspace.Save(ctx, demoWS); err != nil {
		return err
	}

	other := user.New().ID(restOtherUID).
		Name("Other REST User").
		Alias("other-rest-user").
		Email("other-rest@example.com").
		Workspace(restOtherWID).
		MustBuild()
	if err := r.User.Save(ctx, other); err != nil {
		return err
	}

	otherWS := workspace.New().ID(restOtherWID).
		Name("Other Personal").
		Alias("other-rest-personal").
		Members(map[idx.ID[id.User]]workspace.Member{
			restOtherUID: {Role: role.RoleOwner, InvitedBy: restOtherUID},
		}).
		Personal(true).
		MustBuild()
	return r.Workspace.Save(ctx, otherWS)
}
