package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

func runMigration(ctx context.Context, repos *repo.Container) error {
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
	users, err := repos.User.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	processedWorkspaces := make(map[workspace.ID]*workspace.Workspace, len(users))

	// get all workspaces for each user
	for _, u := range users {
		workspaces, err := repos.Workspace.FindByUser(ctx, u.ID())
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

			if wsErr := workspaceMigration(ctx, repos, workspacePkg.ID(w.ID())); wsErr != nil {
				log.Errorf("Failed to migrate workspace %s: %v", w.ID(), wsErr)
				continue
			}
		}

		if userErr := userMigration(ctx, repos, userPkg.ID(u.ID())); userErr != nil && !errors.Is(userErr, rerror.ErrAlreadyExists) {
			log.Errorf("Failed to add alias for user %s: %v", u.ID(), userErr)
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

func userMigration(ctx context.Context, repos *repo.Container, userID userPkg.ID) error {
	usr, err := repos.User.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", userID, err)
	}

	if usr == nil {
		return fmt.Errorf("user %s not found", userID)
	}

	alias := strings.Split(usr.Email(), "@")[0]
	if usr.Alias() == "" {
		usr.SetAlias(alias)
	}

	if usr.Metadata() == nil {
		usr.SetMetadata(userPkg.NewMetadata())
	}

	err = repos.User.Save(ctx, usr)
	if err != nil {
		return fmt.Errorf("failed to save user %s with alias: %w", userID, err)
	}

	return nil
}

func workspaceMigration(ctx context.Context, repos *repo.Container, wsID workspacePkg.ID) error {
	ws, err := repos.Workspace.FindByID(ctx, wsID)
	if err != nil {
		return fmt.Errorf("failed to find workspace %s: %w", wsID, err)
	}

	if ws == nil {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	if ws.Email() == "" {
		ws.UpdateEmail("")
	}

	if ws.Alias() == "" {
		alias := strings.ToLower(strings.ReplaceAll(ws.Name(), " ", "-"))
		ws.UpdateAlias(alias)
	}

	if ws.Metadata() == nil {
		ws.SetMetadata(workspacePkg.NewMetadata())
	}

	err = repos.Workspace.Save(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to save workspace %s with alias: %w", ws.ID(), err)
	}

	return nil
}
