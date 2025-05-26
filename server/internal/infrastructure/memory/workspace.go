package memory

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/util"
)

type Workspace struct {
	data *util.SyncMap[workspace.ID, *workspace.Workspace]
	err  error
}

func NewWorkspace() *Workspace {
	return &Workspace{
		data: &util.SyncMap[workspace.ID, *workspace.Workspace]{},
	}
}

func NewWorkspaceWith(workspaces ...*workspace.Workspace) *Workspace {
	r := NewWorkspace()
	for _, ws := range workspaces {
		r.data.Store(ws.ID(), ws)
	}
	return r
}

func (r *Workspace) FindByID(_ context.Context, v workspace.ID) (*workspace.Workspace, error) {
	if r.err != nil {
		return nil, r.err
	}

	return rerror.ErrIfNil(r.data.Find(func(key workspace.ID, value *workspace.Workspace) bool {
		return key == v
	}), rerror.ErrNotFound)
}

func (r *Workspace) Save(_ context.Context, t *workspace.Workspace) error {
	if r.err != nil {
		return r.err
	}

	r.data.Store(t.ID(), t)
	return nil
}
