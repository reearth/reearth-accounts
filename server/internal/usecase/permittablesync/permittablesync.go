// Package permittablesync keeps a user's Permittable in step with their role in
// a workspace. Every write that changes workspace.Member.Role must also call
// SetWorkspaceRole for that user, inside the same transaction, or the role and
// the permittable disagree.
package permittablesync

import (
	"context"
	"errors"
	"os"

	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// SetWorkspaceRole records roleName as userID's role in workspaceID on the
// user's Permittable, creating the Permittable when the user has none. When
// REEARTH_MOCK_AUTH is "true", a missing role is created on the fly.
func SetWorkspaceRole(
	ctx context.Context,
	roleRepo role.Repo,
	permittableRepo permittable.Repo,
	userID user.ID,
	workspaceID workspace.ID,
	roleName role.RoleType,
) error {
	r, err := roleRepo.FindByName(ctx, string(roleName))
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) && os.Getenv("REEARTH_MOCK_AUTH") == "true" {
			log.Infof("[MockAuth] Auto-creating role for workspace: %s", roleName)
			newRole := role.New().NewID().Name(string(roleName)).MustBuild()
			if saveErr := roleRepo.Save(ctx, *newRole); saveErr != nil {
				return applog.ErrorWithCallerLogging(ctx, "failed to auto-create role", saveErr)
			}
			r = newRole
		} else {
			return err
		}
	}

	p, err := permittableRepo.FindByUserID(ctx, userID)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}

	if p == nil {
		p, err = permittable.New().
			NewID().
			UserID(userID).
			Build()
		if err != nil {
			return err
		}
	}

	p.UpdateWorkspaceRole(workspaceID, r.ID())

	return permittableRepo.Save(ctx, *p)
}
