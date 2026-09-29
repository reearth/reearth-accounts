package interactor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
	"golang.org/x/crypto/bcrypt"
)

type Scim struct {
	repos           *repo.Container
	permittableRepo permittable.Repo
	roleRepo        role.Repo
}

func NewScim(repos *repo.Container) *Scim {
	return &Scim{
		repos:           repos,
		permittableRepo: repos.Permittable,
		roleRepo:        repos.Role,
	}
}

// findRole looks up a role by name, auto-creating it when REEARTH_MOCK_AUTH is set.
func (i *Scim) findRole(ctx context.Context, roleName string) (*role.Role, error) {
	r, err := i.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) && os.Getenv("REEARTH_MOCK_AUTH") == "true" {
			log.Infof("[MockAuth] Auto-creating role: %s", roleName)
			newRole := role.New().NewID().Name(roleName).MustBuild()
			if saveErr := i.roleRepo.Save(ctx, *newRole); saveErr != nil {
				return nil, applog.ErrorWithCallerLogging(ctx, "failed to auto-create role", saveErr)
			}
			return newRole, nil
		}
		return nil, err
	}
	return r, nil
}

// updatePermittable syncs a user's workspace role into the Permittable store.
func (i *Scim) updatePermittable(ctx context.Context, userID user.ID, workspaceID workspace.ID, roleName role.RoleType) error {
	r, err := i.findRole(ctx, string(roleName))
	if err != nil {
		return err
	}

	p, err := i.permittableRepo.FindByUserID(ctx, userID)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}
	if p == nil {
		p, err = permittable.New().NewID().UserID(userID).Build()
		if err != nil {
			return err
		}
	}
	p.UpdateWorkspaceRole(workspaceID, r.ID())
	return i.permittableRepo.Save(ctx, *p)
}

// removePermittable removes a user's workspace role from the Permittable store.
func (i *Scim) removePermittable(ctx context.Context, workspaceID workspace.ID, userID user.ID) error {
	p, err := i.permittableRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return nil
		}
		return applog.ErrorWithCallerLogging(ctx, "failed to fetch permittable", err)
	}
	before := p.UpdatedAt()
	p.RemoveWorkspaceRole(workspaceID)
	if p.UpdatedAt() == before {
		return nil
	}
	return i.permittableRepo.Save(ctx, *p)
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

		if err := i.repos.Workspace.Save(ctx, ws); err != nil {
			return err
		}

		return i.removePermittable(ctx, workspaceID, userID)
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

func (i *Scim) GetScimUser(ctx context.Context, workspaceID workspace.ID, userID user.ID) (*user.User, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	m := ws.Members().User(userID)
	if m == nil || m.Disabled {
		return nil, interfaces.ErrSCIMUserNotFound
	}

	return i.repos.User.FindByID(ctx, userID)
}

// ListScimUsers returns active workspace members, optionally filtered by a
// simple SCIM filter expression. Supported filters:
//   - empty: all active members
//   - `userName eq "<email>"`: members whose account email matches
//   - `externalId eq "<id>"`: members whose SCIM external ID matches
//
// Unsupported filter expressions return ErrOperationDenied.
func (i *Scim) ListScimUsers(ctx context.Context, workspaceID workspace.ID, filter string) ([]*user.User, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	members := ws.Members().Users()

	// Parse filter
	var filterAttr, filterVal string
	if f := strings.TrimSpace(filter); f != "" {
		parts := strings.SplitN(f, " eq ", 2)
		if len(parts) != 2 {
			return nil, interfaces.ErrOperationDenied
		}
		filterAttr = strings.ToLower(strings.TrimSpace(parts[0]))
		filterVal = strings.Trim(strings.TrimSpace(parts[1]), `"`)
		if filterAttr != "username" && filterAttr != "externalid" {
			return nil, interfaces.ErrOperationDenied
		}
	}

	// Collect active user IDs, applying any externalId filter immediately
	userIDs := make(user.IDList, 0, len(members))
	for uid, m := range members {
		if m.Disabled {
			continue
		}
		if filterAttr == "externalid" {
			if m.ExternalID == filterVal {
				userIDs = append(userIDs, uid)
			}
			continue
		}
		userIDs = append(userIDs, uid)
	}

	if len(userIDs) == 0 {
		return nil, nil
	}

	users, err := i.repos.User.FindByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	// Apply userName (email) filter post-fetch
	if filterAttr == "username" {
		filtered := users[:0]
		for _, u := range users {
			if strings.EqualFold(u.Email(), filterVal) {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	return users, nil
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

		roleType := param.Role
		if roleType == "" {
			roleType = role.RoleReader
		}

		// Idempotent: already provisioned by this ExternalID — re-enable if disabled.
		if uid, ok := ws.Members().UserByExternalID(param.ExternalID); ok {
			mem := ws.Members().User(uid)
			if mem != nil && mem.Disabled {
				if err := ws.Members().SetUserDisabled(uid, false); err != nil {
					return nil, err
				}
				if err := i.repos.Workspace.Save(ctx, ws); err != nil {
					return nil, err
				}
				if err := i.updatePermittable(ctx, uid, param.WorkspaceID, ws.Members().UserRole(uid)); err != nil {
					return nil, err
				}
			}
			return i.repos.User.FindByID(ctx, uid)
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
			} else {
				// Existing workspace member: re-enable if disabled.
				mem := ws.Members().User(existingUser.ID())
				if mem != nil && mem.Disabled {
					if err := ws.Members().SetUserDisabled(existingUser.ID(), false); err != nil {
						return nil, err
					}
				}
				// Reconcile role unless demoting the sole active owner.
				if ws.Members().UserRole(existingUser.ID()) != roleType && !ws.Members().IsOnlyOwner(existingUser.ID()) {
					if err := ws.Members().UpdateUserRole(existingUser.ID(), roleType); err != nil {
						return nil, err
					}
				}
			}
			if err := ws.Members().SetUserExternalID(existingUser.ID(), param.ExternalID); err != nil {
				return nil, err
			}
			if err := i.repos.Workspace.Save(ctx, ws); err != nil {
				return nil, err
			}
			if err := i.updatePermittable(ctx, existingUser.ID(), param.WorkspaceID, ws.Members().UserRole(existingUser.ID())); err != nil {
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

		// Bootstrap permittable with the self role and personal workspace owner
		// role, mirroring what normal signup paths do so the user can access
		// their own personal workspace through Cerbos.
		roleSelf, err := i.findRole(ctx, role.RoleSelf.String())
		if err != nil {
			return nil, err
		}
		roleOwner, err := i.findRole(ctx, role.RoleOwner.String())
		if err != nil {
			return nil, err
		}
		personalWsRole := permittable.NewWorkspaceRole(personalWS.ID(), roleOwner.ID())
		perm := permittable.New().NewID().
			RoleIDs([]id.RoleID{roleSelf.ID()}).
			UserID(newUser.ID()).
			WorkspaceRoles([]permittable.WorkspaceRole{personalWsRole}).
			MustBuild()
		if err := i.permittableRepo.Save(ctx, *perm); err != nil {
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
		if err := i.updatePermittable(ctx, newUser.ID(), param.WorkspaceID, roleType); err != nil {
			return nil, err
		}

		return newUser, nil
	})
}

// SyncScimGroup reconciles workspace membership for one IdP group.
//
// NOTE: Group isolation — members from group A are not tracked separately from
// group B. If a user belongs to multiple groups, removing them from one group
// while they remain in another will still disable them. Full group isolation
// requires persisting a per-member group ID, which is a future enhancement.
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
				mem := ws.Members().User(uid)
				// Re-enable if the member was previously deprovisioned.
				if mem != nil && mem.Disabled {
					if err := ws.Members().SetUserDisabled(uid, false); err != nil {
						return err
					}
					if err := i.updatePermittable(ctx, uid, workspaceID, groupRole); err != nil {
						return err
					}
				}
				if ws.Members().UserRole(uid) != groupRole {
					// Guard against demoting the sole owner.
					if ws.Members().IsOnlyOwner(uid) {
						continue
					}
					if err := ws.Members().UpdateUserRole(uid, groupRole); err != nil {
						return err
					}
					if err := i.updatePermittable(ctx, uid, workspaceID, groupRole); err != nil {
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
			} else {
				mem := ws.Members().User(targetUser.ID())
				// Re-enable existing member if disabled.
				if mem != nil && mem.Disabled {
					if err := ws.Members().SetUserDisabled(targetUser.ID(), false); err != nil {
						return err
					}
				}
				if ws.Members().UserRole(targetUser.ID()) != groupRole {
					// Guard against demoting the sole owner: skip the role update
					// but still link the ExternalID and sync Permittable with the
					// retained role so the user can be reconciled on later syncs.
					if !ws.Members().IsOnlyOwner(targetUser.ID()) {
						if err := ws.Members().UpdateUserRole(targetUser.ID(), groupRole); err != nil {
							return err
						}
					}
				}
			}
			if err := ws.Members().SetUserExternalID(targetUser.ID(), m.ExternalID); err != nil {
				return err
			}
			// Use the member's effective role (may differ from groupRole when the
			// sole-owner guard prevented demotion).
			if err := i.updatePermittable(ctx, targetUser.ID(), workspaceID, ws.Members().UserRole(targetUser.ID())); err != nil {
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
			if err := i.removePermittable(ctx, workspaceID, uid); err != nil {
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

		for grp, r := range groupRoleMapping {
			if !r.Valid() || r == role.RoleSelf {
				return nil, fmt.Errorf("%w: invalid role %q for group %q", interfaces.ErrOperationDenied, r, grp)
			}
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
