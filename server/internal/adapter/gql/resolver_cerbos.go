package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearthx/account/accountdomain/user"
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
