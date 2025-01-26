package gql

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/internal/adapter/gql/gqlmodel"
)

func (r *queryResolver) Groups(ctx context.Context) (*gqlmodel.GroupsPayload, error) {
	res, err := usecases(ctx).Group.GetGroups(ctx)
	if err != nil {
		return nil, err
	}

	return &gqlmodel.GroupsPayload{
		Groups: gqlmodel.ToGroups(res),
	}, nil
}
