package gql

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/internal/adapter/gql/gqlmodel"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
)

func (r *mutationResolver) AddGroup(ctx context.Context, input gqlmodel.AddGroupInput) (*gqlmodel.AddGroupPayload, error) {
	group, err := usecases(ctx).Group.AddGroup(ctx, interfaces.AddGroupParam{
		Name: input.Name,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.AddGroupPayload{
		Group: gqlmodel.ToGroup(group),
	}, nil
}

func (r *mutationResolver) UpdateGroup(ctx context.Context, input gqlmodel.UpdateGroupInput) (*gqlmodel.UpdateGroupPayload, error) {
	gid, err := gqlmodel.ToID[id.Group](input.ID)
	if err != nil {
		return nil, err
	}

	group, err := usecases(ctx).Group.UpdateGroup(ctx, interfaces.UpdateGroupParam{
		ID:   gid,
		Name: input.Name,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UpdateGroupPayload{
		Group: gqlmodel.ToGroup(group),
	}, nil
}

func (r *mutationResolver) RemoveGroup(ctx context.Context, input gqlmodel.RemoveGroupInput) (*gqlmodel.RemoveGroupPayload, error) {
	gid, err := gqlmodel.ToID[id.Group](input.ID)
	if err != nil {
		return nil, err
	}

	id, err := usecases(ctx).Group.RemoveGroup(ctx, interfaces.RemoveGroupParam{
		ID: gid,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.RemoveGroupPayload{
		ID: gqlmodel.IDFrom(id),
	}, nil
}
