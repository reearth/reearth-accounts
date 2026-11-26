package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
)

// ActingIntegration returns each workspace through the same converter as
// findByID, so it shows an integration nothing findByID would not.
func (r *queryResolver) ActingIntegration(ctx context.Context) (*gqlmodel.ActingIntegration, error) {
	op := getOperator(ctx)
	ms, err := usecases(ctx).Workspace.ActingIntegrationMemberships(ctx, op)
	if err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	ws := make(workspace.List, 0, len(ms))
	for _, m := range ms {
		ws = append(ws, m.Workspace)
	}
	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		applog.Error(ctx, "[ActingIntegration] failed to build existing user set from workspaces", applog.Err(err))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	res := &gqlmodel.ActingIntegration{
		ID:         gqlmodel.IDFrom(*op.Integration),
		Workspaces: make([]*gqlmodel.IntegrationWorkspace, 0, len(ms)),
	}
	for _, m := range ms {
		w, err := gqlmodel.ToWorkspace(ctx, m.Workspace, exists, r.Photos)
		if err != nil {
			return nil, gqlerror.ReturnAccountsError(ctx, err)
		}
		res.Workspaces = append(res.Workspaces, &gqlmodel.IntegrationWorkspace{Workspace: w, Role: gqlmodel.ToRole(m.Role)})
	}
	return res, nil
}

func (r *mutationResolver) LeaveAllWorkspaces(ctx context.Context) (*gqlmodel.LeaveAllWorkspacesPayload, error) {
	res, err := usecases(ctx).Workspace.LeaveAllAsIntegration(ctx, getOperator(ctx))
	if err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}
	return &gqlmodel.LeaveAllWorkspacesPayload{
		RemovedWorkspaceIds: lo.Map(res.Removed, func(id workspace.ID, _ int) gqlmodel.ID { return gqlmodel.IDFrom(id) }),
		FailedWorkspaceIds:  lo.Map(res.Failed, func(id workspace.ID, _ int) gqlmodel.ID { return gqlmodel.IDFrom(id) }),
	}, nil
}
