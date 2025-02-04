package interactor

import (
	"context"
	"errors"

	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Group struct {
	permittableRepo repo.Permittable
	groupRepo       repo.Group
	transaction     usecasex.Transaction
}

func NewGroup(r *repo.Container) interfaces.Group {
	return &Group{
		permittableRepo: r.Permittable,
		groupRepo:       r.Group,
		transaction:     r.Transaction,
	}
}

func (i *Group) GetGroups(ctx context.Context) (group.List, error) {
	groups, err := i.groupRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

func (i *Group) AddGroup(ctx context.Context, param interfaces.AddGroupParam) (*group.Group, error) {
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

	group, err := group.New().
		NewID().
		Name(param.Name).
		Build()
	if err != nil {
		return nil, err
	}

	if err := i.groupRepo.Save(ctx, *group); err != nil {
		return nil, err
	}

	tx.Commit()
	return group, nil
}

func (i *Group) UpdateGroup(ctx context.Context, param interfaces.UpdateGroupParam) (*group.Group, error) {
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

	group, err := i.groupRepo.FindByID(ctx, param.ID)
	if err != nil {
		return nil, err
	}

	group.Rename(param.Name)

	if err := i.groupRepo.Save(ctx, *group); err != nil {
		return nil, err
	}

	tx.Commit()
	return group, nil
}

func (i *Group) RemoveGroup(ctx context.Context, param interfaces.RemoveGroupParam) (id.GroupID, error) {
	tx, err := i.transaction.Begin(ctx)
	if err != nil {
		return id.GroupID{}, err
	}

	ctx = tx.Context()
	defer func() {
		if err2 := tx.End(ctx); err == nil && err2 != nil {
			err = err2
		}
	}()

	targetGroup, err := i.groupRepo.FindByID(ctx, param.ID)
	if err != nil {
		return id.GroupID{}, err
	}

	// Check if the group can be removed
	permittables, err := i.permittableRepo.FindByGroupID(ctx, param.ID)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return id.GroupID{}, err
	}
	if len(permittables) > 0 {
		return id.GroupID{}, errors.New("group is in use")
	}

	if err := i.groupRepo.Remove(ctx, targetGroup.ID()); err != nil {
		return id.GroupID{}, err
	}

	tx.Commit()
	return param.ID, nil
}
