package interactor

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type Permittable struct {
	permittableRepo permittable.Repo
	roleRepo        role.Repo
	userRepo        user.Repo
}

func NewPermittable(r *repo.Container) interfaces.Permittable {
	return &Permittable{
		permittableRepo: r.Permittable,
		roleRepo:        r.Role,
		userRepo:        r.User,
	}
}

func (i *Permittable) GetUsersWithRoles(ctx context.Context) (user.List, map[user.ID]role.List, error) {
	// Find all users
	users, err := i.userRepo.FindAll(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(users) == 0 {
		return user.List{}, map[user.ID]role.List{}, nil
	}

	userIds := make([]user.ID, 0, len(users))
	for _, u := range users {
		userIds = append(userIds, u.ID())
	}

	// Find permittables by user IDs
	permittables, err := i.permittableRepo.FindByUserIDs(ctx, userIds)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, nil, err
	}
	if len(permittables) == 0 {
		return users, map[user.ID]role.List{}, nil
	}

	roleIdsMap := make(map[role.ID]struct{})
	roleIds := make([]role.ID, 0)
	for _, permittable := range permittables {
		for _, roleId := range permittable.RoleIDs() {
			if _, exists := roleIdsMap[roleId]; !exists {
				roleIdsMap[roleId] = struct{}{}
				roleIds = append(roleIds, roleId)
			}
		}
	}

	// Find roles by role IDs
	roles, err := i.roleRepo.FindByIDs(ctx, roleIds)
	if err != nil {
		return nil, nil, err
	}

	roleMap := make(map[role.ID]*role.Role)
	for _, r := range roles {
		roleMap[r.ID()] = r
	}

	userRoleMap := make(map[user.ID]role.List)
	for _, permittable := range permittables {
		userRoles := make(role.List, 0)
		for _, roleId := range permittable.RoleIDs() {
			if r, exists := roleMap[roleId]; exists {
				userRoles = append(userRoles, r)
			}
		}
		userRoleMap[permittable.UserID()] = userRoles
	}

	return users, userRoleMap, nil
}
