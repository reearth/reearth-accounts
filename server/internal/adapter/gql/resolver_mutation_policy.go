package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
)

func (r *mutationResolver) AcceptPolicies(ctx context.Context, input []*gqlmodel.PolicyAcceptanceInput) (*gqlmodel.AcceptPoliciesPayload, error) {
	if err := denyServiceCaller(ctx); err != nil {
		return nil, err
	}
	u, err := usecases(ctx).Policy.Accept(ctx, gqlmodel.ToPolicyRefInputs(input), adapter.Client(ctx), getOperator(ctx))
	if err != nil {
		return nil, err
	}
	return &gqlmodel.AcceptPoliciesPayload{Me: gqlmodel.ToMe(u)}, nil
}
