package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/workspace"
)

type Workspace interface {
	FindByID(ctx context.Context, id workspace.ID) (*workspace.Workspace, error)
	Save(ctx context.Context, ws *workspace.Workspace) error
}
