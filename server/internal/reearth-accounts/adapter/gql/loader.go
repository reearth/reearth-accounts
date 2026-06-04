package gql

import (
	"context"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
)

const (
	dataLoaderWait     = 1 * time.Millisecond
	dataLoaderMaxBatch = 100
)

type Loaders struct {
	usecases  interfaces.Container
	Workspace *WorkspaceLoader
	User      *UserLoader
}

type DataLoaders struct {
	Workspace WorkspaceDataLoader
	User      UserDataLoader
}

func NewLoaders(usecases *interfaces.Container, storage gateway.Storage) *Loaders {
	if usecases == nil {
		return nil
	}
	return &Loaders{
		usecases:  *usecases,
		Workspace: NewWorkspaceLoader(usecases.Workspace, storage),
		User:      NewUserLoader(usecases.User),
	}
}

func (l Loaders) DataLoadersWith(ctx context.Context, enabled bool) *DataLoaders {
	if enabled {
		return l.DataLoaders(ctx)
	}
	return l.OrdinaryDataLoaders(ctx)
}

func (l Loaders) DataLoaders(ctx context.Context) *DataLoaders {
	return &DataLoaders{
		Workspace: l.Workspace.DataLoader(ctx),
		User:      l.User.DataLoader(ctx),
	}
}

func (l Loaders) OrdinaryDataLoaders(ctx context.Context) *DataLoaders {
	return &DataLoaders{
		Workspace: l.Workspace.OrdinaryDataLoader(ctx),
		User:      l.User.OrdinaryDataLoader(ctx),
	}
}
