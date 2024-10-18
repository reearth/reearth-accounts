package gql

import (
	"context"

	"github.com/reearth/reearth-account/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-account/internal/usecase/interfaces"
)

func (r *queryResolver) CheckPermission(ctx context.Context, input gqlmodel.CheckPermissionInput) (*gqlmodel.CheckPermissionPayload, error) {
	res, err := usecases(ctx).Cerbos.CheckPermission(ctx, interfaces.CheckPermissionParam{
		Service:  input.Service,
		Resource: input.Resource,
		Action:   input.Action,
	}, getUser(ctx))
	if err != nil {
		return nil, err
	}

	return &gqlmodel.CheckPermissionPayload{
		Allowed: res.Allowed,
	}, nil
}
