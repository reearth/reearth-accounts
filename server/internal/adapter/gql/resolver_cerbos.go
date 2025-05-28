package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

func (r *queryResolver) CheckPermission(ctx context.Context, input gqlmodel.CheckPermissionInput) (*gqlmodel.CheckPermissionPayload, error) {
	userId, err := user.IDFrom(input.UserID)
	if err != nil {
		return nil, err
	}

	res, err := usecases(ctx).Cerbos.CheckPermission(ctx, userId, interfaces.CheckPermissionParam{
		Service:  input.Service,
		Resource: input.Resource,
		Action:   input.Action,
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.CheckPermissionPayload{
		Allowed: res.Allowed,
	}, nil
}
