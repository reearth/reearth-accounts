package gql

import (
	"context"

	"github.com/reearth/reearth-account/internal/adapter/gql/gqlmodel"
)

func (r *queryResolver) Roles(ctx context.Context) (*gqlmodel.RolesPayload, error) {
	res, err := usecases(ctx).Role.GetRoles(ctx)
	if err != nil {
		return nil, err
	}

	return &gqlmodel.RolesPayload{
		Roles: gqlmodel.ToRoles(res),
	}, nil
}
