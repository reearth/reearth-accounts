package gql

import (
	"context"

	"github.com/reearth/reearth-account/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-account/internal/usecase/interfaces"
	"github.com/reearth/reearth-account/pkg/id"
)

func (r *mutationResolver) AddRole(ctx context.Context, input gqlmodel.AddRoleInput) (*gqlmodel.AddRolePayload, error) {
	role, err := usecases(ctx).Role.AddRole(ctx, interfaces.AddRoleParam{
		Name: input.Name,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.AddRolePayload{
		Role: gqlmodel.ToRoleForAuthorization(role),
	}, nil
}

func (r *mutationResolver) UpdateRole(ctx context.Context, input gqlmodel.UpdateRoleInput) (*gqlmodel.UpdateRolePayload, error) {
	rid, err := gqlmodel.ToID[id.Role](input.ID)
	if err != nil {
		return nil, err
	}

	role, err := usecases(ctx).Role.UpdateRole(ctx, interfaces.UpdateRoleParam{
		ID:   rid,
		Name: input.Name,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UpdateRolePayload{
		Role: gqlmodel.ToRoleForAuthorization(role),
	}, nil
}

func (r *mutationResolver) RemoveRole(ctx context.Context, input gqlmodel.RemoveRoleInput) (*gqlmodel.RemoveRolePayload, error) {
	rid, err := gqlmodel.ToID[id.Role](input.ID)
	if err != nil {
		return nil, err
	}

	id, err := usecases(ctx).Role.RemoveRole(ctx, interfaces.RemoveRoleParam{
		ID: rid,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.RemoveRolePayload{
		ID: gqlmodel.IDFrom(id),
	}, nil
}
