package interactor

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Permittable struct {
	permittableRepo repo.Permittable
	roleRepo        repo.Role
	userRepo        repo.User
	transaction     usecasex.Transaction
}

func NewPermittable(r *repo.Container) interfaces.Permittable {
	return &Permittable{
		permittableRepo: r.Permittable,
		roleRepo:        r.Role,
		userRepo:        r.User,
		transaction:     r.Transaction,
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

func (i *Permittable) UpdatePermittable(ctx context.Context, param interfaces.UpdatePermittableParam) (*permittable.Permittable, error) {
	tx, err := i.transaction.Begin(ctx)
	if err != nil {
		return nil, err
	}

	ctx = tx.Context()
	defer func() {
		if err2 := tx.End(ctx); err == nil && err2 != nil {
			err = err2
		}
	}()

	targetPermittable, err := i.permittableRepo.FindByUserID(ctx, param.UserID)
	if err != nil && err != rerror.ErrNotFound {
		return nil, err
	}

	var u *permittable.Permittable
	if targetPermittable != nil {
		targetPermittable.EditRoleIDs(param.RoleIDs)
		u = targetPermittable
	} else {
		u, err = permittable.New().
			NewID().
			UserID(param.UserID).
			RoleIDs(param.RoleIDs).
			Build()
		if err != nil {
			return nil, err
		}
	}

	if err := i.permittableRepo.Save(ctx, *u); err != nil {
		return nil, err
	}

	tx.Commit()
	return u, nil
}
