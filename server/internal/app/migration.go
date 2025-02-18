package app

import (
	"context"
	"fmt"

	"github.com/eukarya-inc/reearth-accounts/internal/usecase/repo"
	"github.com/eukarya-inc/reearth-accounts/pkg/id"
	"github.com/eukarya-inc/reearth-accounts/pkg/permittable"
	"github.com/eukarya-inc/reearth-accounts/pkg/role"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/account/accountdomain/workspace"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

func runMigration(ctx context.Context, repos *repo.Container, acRepos *accountrepo.Container) error {
	log.Info("Starting migration...")

	// check and create maintainer role
	roles, err := repos.Role.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get roles: %w", err)
	}

	var maintainerRole *role.Role
	for _, r := range roles {
		if r.Name() == "maintainer" {
			maintainerRole = r
			log.Info("Found maintainer role")
			break
		}
	}

	if maintainerRole == nil {
		r, err := role.New().
			NewID().
			Name("maintainer").
			Build()
		if err != nil {
			return fmt.Errorf("failed to create maintainer role domain: %w", err)
		}

		err = repos.Role.Save(ctx, *r)
		if err != nil {
			return fmt.Errorf("failed to save maintainer role: %w", err)
		}

		maintainerRole = r
		log.Info("Created maintainer role")
	}

	// get all users
	users, err := acRepos.User.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	processedWorkspaces := make(map[workspace.ID]*workspace.Workspace, len(users))

	// get all workspaces for each user
	for _, u := range users {
		workspaces, err := acRepos.Workspace.FindByUser(ctx, u.ID())
		if err != nil {
			log.Errorf("Failed to get workspaces for user %s: %v", u.ID, err)
			continue
		}

		for _, w := range workspaces {
			if _, exists := processedWorkspaces[w.ID()]; exists {
				continue
			}
			processedWorkspaces[w.ID()] = w

			// Process workspace members and integrations
			if err := processWorkspaceMembers(ctx, repos, w, maintainerRole.ID()); err != nil {
				log.Errorf("Failed to process workspace %s: %v", w.ID, err)
			}
		}
	}

	log.Info("Migration completed successfully")
	return nil
}

func processWorkspaceMembers(ctx context.Context, repos *repo.Container, w *workspace.Workspace, maintainerRoleID role.ID) error {
	members := w.Members()

	// Process users
	for _, userID := range members.UserIDs() {
		if err := ensureMaintainerRole(ctx, repos, userID, maintainerRoleID); err != nil {
			log.Errorf("Failed to process user %s: %v", userID, err)
		}
	}

	// Process integrations
	for _, integrationID := range members.IntegrationIDs() {
		userID, err := user.IDFrom(integrationID.String())
		if err != nil {
			log.Errorf("Failed to process integration %s: %v", integrationID, err)
			continue
		}
		if err := ensureMaintainerRole(ctx, repos, userID, maintainerRoleID); err != nil {
			log.Errorf("Failed to process integration %s: %v", integrationID, err)
		}
	}

	return nil
}

func ensureMaintainerRole(ctx context.Context, repos *repo.Container, userID user.ID, maintainerRoleID role.ID) error {
	var p *permittable.Permittable
	var err error

	p, err = repos.Permittable.FindByUserID(ctx, userID)
	if err != nil && err != rerror.ErrNotFound {
		return err
	}

	if hasRole(p, maintainerRoleID) {
		return nil
	}

	if p == nil {
		p, err = permittable.New().
			NewID().
			UserID(userID).
			RoleIDs([]id.RoleID{maintainerRoleID}).
			Build()
		if err != nil {
			return err
		}
	} else {
		p.EditRoleIDs(append(p.RoleIDs(), maintainerRoleID))
	}

	err = repos.Permittable.Save(ctx, *p)
	if err != nil {
		return err
	}

	return nil
}

func hasRole(p *permittable.Permittable, roleID role.ID) bool {
	if p == nil {
		return false
	}

	for _, r := range p.RoleIDs() {
		if r == roleID {
			return true
		}
	}
	return false
}
