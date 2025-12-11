package memory

import (
	"context"
	"slices"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
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

func (r *Workspace) Filtered(f repo.WorkspaceFilter) repo.Workspace {
	return &Workspace{
		data: r.data,
		err:  r.err,
	}
}

func (r *Workspace) FindByUser(_ context.Context, i id.UserID) (workspace.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	return rerror.ErrIfNil(r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		return value.Members().HasUser(i)
	}), rerror.ErrNotFound)
}

func (r *Workspace) FindByUserWithPagination(ctx context.Context, id user.ID, pagination *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	if r.err != nil {
		return nil, nil, r.err
	}

	workspaces := workspace.List(r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		return value.Members().HasUser(id)
	}))

	if len(workspaces) == 0 {
		return nil, nil, rerror.ErrNotFound
	}

	startCursor, endCursor := usecasex.Cursor(workspaces[0].ID().String()), usecasex.Cursor(workspaces[len(workspaces)-1].ID().String())

	return workspaces, usecasex.NewPageInfo(
		int64(len(workspaces)),
		&startCursor,
		&endCursor,
		true,
		true,
	), nil
}

func (r *Workspace) FindByIntegration(_ context.Context, i workspace.IntegrationID) (workspace.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	return rerror.ErrIfNil(r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		return value.Members().HasIntegration(i)
	}), rerror.ErrNotFound)
}

// FindByIntegrations finds workspace list based on integrations IDs
func (r *Workspace) FindByIntegrations(_ context.Context, ids workspace.IntegrationIDList) (workspace.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	res := r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		return slices.ContainsFunc(ids, value.Members().HasIntegration)
	})

	slices.SortFunc(res, func(a, b *workspace.Workspace) int { return a.ID().Compare(b.ID()) })

	return res, nil
}

func (r *Workspace) FindByIDs(_ context.Context, ids workspace.IDList) (workspace.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	res := r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		return ids.Has(key)
	})
	slices.SortFunc(res, func(a, b *workspace.Workspace) int { return a.ID().Compare(b.ID()) })
	return res, nil
}

func (r *Workspace) FindByID(_ context.Context, v workspace.ID) (*workspace.Workspace, error) {
	if r.err != nil {
		return nil, r.err
	}

	return rerror.ErrIfNil(r.data.Find(func(key workspace.ID, value *workspace.Workspace) bool {
		return key == v
	}), rerror.ErrNotFound)
}

func (r *Workspace) FindByName(_ context.Context, name string) (*workspace.Workspace, error) {
	if r.err != nil {
		return nil, r.err
	}
	if name == "" {
		return nil, rerror.ErrNotFound
	}
	return rerror.ErrIfNil(r.data.Find(func(key workspace.ID, value *workspace.Workspace) bool {
		return value.Name() == name
	}), rerror.ErrNotFound)
}

func (r *Workspace) FindByAlias(_ context.Context, alias string) (*workspace.Workspace, error) {
	if r.err != nil {
		return nil, r.err
	}
	if alias == "" {
		return nil, rerror.ErrNotFound
	}
	return rerror.ErrIfNil(r.data.Find(func(key workspace.ID, value *workspace.Workspace) bool {
		return value.Alias() == alias
	}), rerror.ErrNotFound)
}

func (r *Workspace) FindByAliases(_ context.Context, aliases []string) (workspace.List, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	aliasSet := make(map[string]struct{})
	for _, a := range aliases {
		aliasSet[a] = struct{}{}
	}
	res := r.data.FindAll(func(key workspace.ID, value *workspace.Workspace) bool {
		_, ok := aliasSet[value.Alias()]
		return ok
	})
	slices.SortFunc(res, func(a, b *workspace.Workspace) int { return a.ID().Compare(b.ID()) })
	return res, nil
}

func (r *Workspace) Create(_ context.Context, t *workspace.Workspace) error {
	if r.err != nil {
		return r.err
	}

	if _, ok := r.data.Load(t.ID()); ok {
		return rerror.ErrAlreadyExists
	}

	r.data.Store(t.ID(), t)
	return nil
}

func (r *Workspace) Save(_ context.Context, t *workspace.Workspace) error {
	if r.err != nil {
		return r.err
	}

	r.data.Store(t.ID(), t)
	return nil
}

func (r *Workspace) SaveAll(_ context.Context, workspaces workspace.List) error {
	if r.err != nil {
		return r.err
	}

	for _, t := range workspaces {
		r.data.Store(t.ID(), t)
	}
	return nil
}

func (r *Workspace) Remove(_ context.Context, wid workspace.ID) error {
	if r.err != nil {
		return r.err
	}

	r.data.Delete(wid)
	return nil
}

func (r *Workspace) RemoveAll(_ context.Context, ids workspace.IDList) error {
	if r.err != nil {
		return r.err
	}

	for _, wid := range ids {
		r.data.Delete(wid)
	}
	return nil
}

func SetWorkspaceError(r repo.Workspace, err error) {
	r.(*Workspace).err = err
}
