package gql

import (
	"context"

	"github.com/reearth/reearth-account/internal/adapter/gql/gqlmodel"
)

func (r *queryResolver) GetUsersWithRoles(ctx context.Context) (*gqlmodel.GetUsersWithRolesPayload, error) {
	users, userRoleMap, err := usecases(ctx).Permittable.GetUsersWithRoles(ctx)
	if err != nil {
		return nil, err
	}

	return &gqlmodel.GetUsersWithRolesPayload{
		UsersWithRoles: gqlmodel.ToUsersWithRoles(users, userRoleMap),
	}, nil
}
