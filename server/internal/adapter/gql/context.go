package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type ContextKey string

const (
	contextLoaders     ContextKey = "loaders"
	contextDataloaders ContextKey = "dataloaders"
)

func AttachUsecases(ctx context.Context, u *interfaces.Container, photos adapter.PhotoSigner, enableDataLoaders bool) context.Context {
	loaders := NewLoaders(u, photos)
	dataloaders := loaders.DataLoadersWith(ctx, enableDataLoaders)

	ctx = adapter.AttachUsecases(ctx, u)
	ctx = context.WithValue(ctx, contextLoaders, loaders)
	ctx = context.WithValue(ctx, contextDataloaders, dataloaders)

	return ctx
}

func getUser(ctx context.Context) *user.User {
	return adapter.User(ctx)
}

func getOperator(ctx context.Context) *workspace.Operator {
	return adapter.Operator(ctx)
}

func usecases(ctx context.Context) *interfaces.Container {
	return adapter.Usecases(ctx)
}

func loaders(ctx context.Context) *Loaders {
	return ctx.Value(contextLoaders).(*Loaders)
}

func dataloaders(ctx context.Context) *DataLoaders {
	return ctx.Value(contextDataloaders).(*DataLoaders)
}

// denyClient keeps the user directory closed to a client call that carries an
// operator, in case a transport skips the client gate: a client is scoped to the
// workspaces of what it acts for (ADR-0002 decision 5). An anonymous client call
// has no operator and gets what a request with no token gets.
func denyClient(ctx context.Context) error {
	if getOperator(ctx).ViaClient() {
		return interfaces.ErrInvalidOperator
	}
	return nil
}
