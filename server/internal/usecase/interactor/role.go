package interactor

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Role struct {
	permittableRepo repo.Permittable
	roleRepo        repo.Role
	transaction     usecasex.Transaction
}

func NewRole(r *repo.Container) interfaces.Role {
	return &Role{
		permittableRepo: r.Permittable,
		roleRepo:        r.Role,
		transaction:     r.Transaction,
	}
}

func (i *Role) GetRoles(ctx context.Context) (role.List, error) {
	roles, err := i.roleRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (i *Role) AddRole(ctx context.Context, param interfaces.AddRoleParam) (*role.Role, error) {
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

	role, err := role.New().
		NewID().
		Name(param.Name).
		Build()
	if err != nil {
		return nil, err
	}

	if err := i.roleRepo.Save(ctx, *role); err != nil {
		return nil, err
	}

	tx.Commit()
	return role, nil
}

func (i *Role) UpdateRole(ctx context.Context, param interfaces.UpdateRoleParam) (*role.Role, error) {
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

	role, err := i.roleRepo.FindByID(ctx, param.ID)
	if err != nil {
		return nil, err
	}

	role.Rename(param.Name)

	if err := i.roleRepo.Save(ctx, *role); err != nil {
		return nil, err
	}

	tx.Commit()
	return role, nil
}

func (i *Role) RemoveRole(ctx context.Context, param interfaces.RemoveRoleParam) (id.RoleID, error) {
	tx, err := i.transaction.Begin(ctx)
	if err != nil {
		return id.RoleID{}, err
	}

	ctx = tx.Context()
	defer func() {
		if err2 := tx.End(ctx); err == nil && err2 != nil {
			err = err2
		}
	}()

	targetRole, err := i.roleRepo.FindByID(ctx, param.ID)
	if err != nil {
		return id.RoleID{}, err
	}

	// Check if the role can be removed
	permittables, err := i.permittableRepo.FindByRoleID(ctx, param.ID)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return id.RoleID{}, err
	}
	if len(permittables) > 0 {
		return id.RoleID{}, errors.New("role is in use")
	}

	if err := i.roleRepo.Remove(ctx, targetRole.ID()); err != nil {
		return id.RoleID{}, err
	}

	tx.Commit()
	return param.ID, nil
}
