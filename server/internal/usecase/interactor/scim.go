package interactor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"golang.org/x/crypto/bcrypt"
)

type Scim struct {
	repos *repo.Container
}

func NewScim(repos *repo.Container) *Scim {
	return &Scim{repos: repos}
}

func (i *Scim) DeprovisionScimUser(ctx context.Context, workspaceID workspace.ID, externalID string) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		userID, ok := ws.Members().UserByExternalID(externalID)
		if !ok {
			return interfaces.ErrSCIMUserNotFound
		}

		if ws.Members().IsOnlyOwner(userID) {
			return interfaces.ErrOwnerCannotLeaveTheWorkspace
		}

		if err := ws.Members().SetUserDisabled(userID, true); err != nil {
			return err
		}

		return i.repos.Workspace.Save(ctx, ws)
	})
}

func (i *Scim) DeprovisionScimUserByUserID(ctx context.Context, workspaceID workspace.ID, userID user.ID) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		if !ws.Members().HasUser(userID) {
			return interfaces.ErrSCIMUserNotFound
		}

		if ws.Members().IsOnlyOwner(userID) {
			return interfaces.ErrOwnerCannotLeaveTheWorkspace
		}

		if err := ws.Members().SetUserDisabled(userID, true); err != nil {
			return err
		}

		return i.repos.Workspace.Save(ctx, ws)
	})
}

func (i *Scim) GenerateScimToken(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (string, error) {
	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithMaintainableWorkspaces(workspaceID), func(ctx context.Context) (string, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return "", err
		}

		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		plaintext := base64.RawURLEncoding.EncodeToString(raw)

		hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}

		cfg := ws.ScimConfig()
		if cfg == nil {
			cfg = workspace.NewScimConfig()
		}
		cfg.SetTokenHash(string(hash))
		ws.SetScimConfig(cfg)

		if err := i.repos.Workspace.Save(ctx, ws); err != nil {
			return "", err
		}

		return plaintext, nil
	})
}

func (i *Scim) DeleteScimGroup(ctx context.Context, workspaceID workspace.ID, groupName string) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		cfg := ws.ScimConfig()
		if cfg == nil {
			return nil
		}

		mapping := cfg.GroupRoleMapping()
		delete(mapping, groupName)
		cfg.SetGroupRoleMapping(mapping)
		ws.SetScimConfig(cfg)

		return i.repos.Workspace.Save(ctx, ws)
	})
}

func (i *Scim) GetScimConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (*workspace.ScimConfig, error) {
	return Run1(ctx, operator, i.repos, Usecase().WithMaintainableWorkspaces(workspaceID), func(ctx context.Context) (*workspace.ScimConfig, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		cfg := ws.ScimConfig()
		if cfg == nil {
			return nil, nil
		}

		if cfg.TokenHash() != "" {
			cfg.SetTokenHash("***")
		}

		return cfg, nil
	})
}

func (i *Scim) RevokeScimToken(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) error {
	return Run0(ctx, operator, i.repos, Usecase().Transaction().WithMaintainableWorkspaces(workspaceID), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		cfg := ws.ScimConfig()
		if cfg == nil {
			return nil
		}
		cfg.SetEnabled(false)
		cfg.SetTokenHash("")
		ws.SetScimConfig(cfg)

		return i.repos.Workspace.Save(ctx, ws)
	})
}

func (i *Scim) GetScimUser(ctx context.Context, workspaceID workspace.ID, userID user.ID) (*user.User, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if !ws.Members().HasUser(userID) {
		return nil, interfaces.ErrSCIMUserNotFound
	}

	return i.repos.User.FindByID(ctx, userID)
}

func (i *Scim) ListScimUsers(ctx context.Context, workspaceID workspace.ID, _ string) ([]*user.User, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	members := ws.Members().Users()
	userIDs := make(user.IDList, 0, len(members))
	for uid, m := range members {
		if !m.Disabled {
			userIDs = append(userIDs, uid)
		}
	}

	if len(userIDs) == 0 {
		return nil, nil
	}

	return i.repos.User.FindByIDs(ctx, userIDs)
}

func (i *Scim) ProvisionScimUser(ctx context.Context, param interfaces.ProvisionScimUserParam) (*user.User, error) {
	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, param.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if !ws.ScimConfig().Enabled() {
			return nil, interfaces.ErrSCIMNotEnabled
		}

		// Idempotent: already provisioned by this ExternalID
		if uid, ok := ws.Members().UserByExternalID(param.ExternalID); ok {
			return i.repos.User.FindByID(ctx, uid)
		}

		roleType := param.Role
		if roleType == "" {
			roleType = role.RoleReader
		}

		// User may already exist from JIT or manual invite
		existingUser, err := i.repos.User.FindByEmail(ctx, param.Email)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		}

		if existingUser != nil {
			if !ws.Members().HasUser(existingUser.ID()) {
				if err := ws.Members().Join(existingUser, roleType, existingUser.ID()); err != nil {
					return nil, err
				}
			}
			if err := ws.Members().SetUserExternalID(existingUser.ID(), param.ExternalID); err != nil {
				return nil, err
			}
			if err := i.repos.Workspace.Save(ctx, ws); err != nil {
				return nil, err
			}
			return existingUser, nil
		}

		// Create new user + personal workspace
		newUser, personalWS, err := workspace.Init(workspace.InitParams{
			Email: param.Email,
			Name:  param.Name,
		})
		if err != nil {
			return nil, err
		}
		if err := i.repos.User.Create(ctx, newUser); err != nil {
			return nil, err
		}
		if err := i.repos.Workspace.Save(ctx, personalWS); err != nil {
			return nil, err
		}

		if err := ws.Members().Join(newUser, roleType, newUser.ID()); err != nil {
			return nil, err
		}
		if err := ws.Members().SetUserExternalID(newUser.ID(), param.ExternalID); err != nil {
			return nil, err
		}
		if err := i.repos.Workspace.Save(ctx, ws); err != nil {
			return nil, err
		}

		return newUser, nil
	})
}

func (i *Scim) SyncScimGroup(ctx context.Context, workspaceID workspace.ID, _, groupName string, members []interfaces.ScimGroupMember) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return err
		}
		if !ws.ScimConfig().Enabled() {
			return interfaces.ErrSCIMNotEnabled
		}

		// Determine role for this group
		groupRole := role.RoleReader
		if mapping := ws.ScimConfig().GroupRoleMapping(); mapping != nil {
			if r, ok := mapping[groupName]; ok {
				groupRole = r
			}
		}

		// Index incoming members by ExternalID
		incomingExtIDs := make(map[string]struct{}, len(members))
		for _, m := range members {
			incomingExtIDs[m.ExternalID] = struct{}{}
		}

		// Provision or update role for each incoming member
		for _, m := range members {
			uid, exists := ws.Members().UserByExternalID(m.ExternalID)
			if exists {
				if ws.Members().UserRole(uid) != groupRole {
					if err := ws.Members().UpdateUserRole(uid, groupRole); err != nil {
						return err
					}
				}
				continue
			}

			// Resolve user from provided UserID
			var targetUser *user.User
			if m.UserID != nil {
				targetUser, err = i.repos.User.FindByID(ctx, *m.UserID)
				if err != nil && !errors.Is(err, rerror.ErrNotFound) {
					return err
				}
			}
			if targetUser == nil {
				continue
			}

			if !ws.Members().HasUser(targetUser.ID()) {
				if err := ws.Members().Join(targetUser, groupRole, targetUser.ID()); err != nil {
					return err
				}
			} else if ws.Members().UserRole(targetUser.ID()) != groupRole {
				if err := ws.Members().UpdateUserRole(targetUser.ID(), groupRole); err != nil {
					return err
				}
			}
			if err := ws.Members().SetUserExternalID(targetUser.ID(), m.ExternalID); err != nil {
				return err
			}
		}

		// Soft-disable members no longer in the group
		for uid, mem := range ws.Members().Users() {
			if mem.ExternalID == "" {
				continue
			}
			if _, ok := incomingExtIDs[mem.ExternalID]; ok {
				continue
			}
			if ws.Members().IsOnlyOwner(uid) {
				continue
			}
			if err := ws.Members().SetUserDisabled(uid, true); err != nil {
				return err
			}
		}

		return i.repos.Workspace.Save(ctx, ws)
	})
}

func (i *Scim) UpdateScimConfig(ctx context.Context, workspaceID workspace.ID, enabled bool, groupRoleMapping map[string]role.RoleType, operator *workspace.Operator) (*workspace.ScimConfig, error) {
	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithMaintainableWorkspaces(workspaceID), func(ctx context.Context) (*workspace.ScimConfig, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		cfg := ws.ScimConfig()
		if cfg == nil {
			cfg = workspace.NewScimConfig()
		}
		cfg.SetEnabled(enabled)
		cfg.SetGroupRoleMapping(groupRoleMapping)
		ws.SetScimConfig(cfg)

		if err := i.repos.Workspace.Save(ctx, ws); err != nil {
			return nil, err
		}

		result := ws.ScimConfig()
		if result != nil && result.TokenHash() != "" {
			result.SetTokenHash("***")
		}
		return result, nil
	})
}
