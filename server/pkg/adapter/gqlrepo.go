package adapter

import (
	"context"
	"fmt"

	"github.com/reearth/reearth-accounts/server/pkg/gqlclient"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/user"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/workspace"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	accountsUser "github.com/reearth/reearth-accounts/server/pkg/user"
	accountsWorkspace "github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

// WorkspaceRepoAdapter adapts gqlclient.WorkspaceRepo to repo.Workspace interface
type WorkspaceRepoAdapter struct {
	inner  workspace.WorkspaceRepo
	filter repo.WorkspaceFilter
}

func NewWorkspaceRepo(gqlClient *gqlclient.Client) repo.Workspace {
	return &WorkspaceRepoAdapter{
		inner: gqlClient.WorkspaceRepo,
	}
}

func (a *WorkspaceRepoAdapter) Filtered(f repo.WorkspaceFilter) repo.Workspace {
	return &WorkspaceRepoAdapter{
		inner:  a.inner,
		filter: a.filter.Merge(f),
	}
}

func (a *WorkspaceRepoAdapter) FindByID(ctx context.Context, wsid id.WorkspaceID) (*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByName(ctx context.Context, name string) (*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByAlias(ctx context.Context, alias string) (*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByIDs(ctx context.Context, ids id.WorkspaceIDList) ([]*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByUser(ctx context.Context, uid id.UserID) ([]*accountsWorkspace.Workspace, error) {
	workspaces, err := a.inner.FindByUser(ctx, uid.String())
	if err != nil {
		return nil, err
	}
	// Filter based on permissions
	if a.filter.Readable != nil {
		filtered := make([]*accountsWorkspace.Workspace, 0, len(workspaces))
		for _, ws := range workspaces {
			if a.filter.CanRead(ws.ID()) {
				filtered = append(filtered, ws)
			}
		}
		return filtered, nil
	}
	return workspaces, nil
}

func (a *WorkspaceRepoAdapter) FindByUserWithPagination(ctx context.Context, uid id.UserID, pagination *usecasex.Pagination) ([]*accountsWorkspace.Workspace, *usecasex.PageInfo, error) {
	return nil, nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByIntegration(ctx context.Context, iid id.IntegrationID) ([]*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) FindByIntegrations(ctx context.Context, iids id.IntegrationIDList) ([]*accountsWorkspace.Workspace, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) CheckWorkspaceAliasUnique(ctx context.Context, wsid id.WorkspaceID, alias string) error {
	return rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) Create(ctx context.Context, ws *accountsWorkspace.Workspace) error {
	return rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) Save(ctx context.Context, ws *accountsWorkspace.Workspace) error {
	return rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) SaveAll(ctx context.Context, wsList []*accountsWorkspace.Workspace) error {
	return rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) Remove(ctx context.Context, wsid id.WorkspaceID) error {
	return rerror.ErrNotImplemented
}

func (a *WorkspaceRepoAdapter) RemoveAll(ctx context.Context, ids id.WorkspaceIDList) error {
	return rerror.ErrNotImplemented
}

// UserRepoAdapter adapts gqlclient.UserRepo to repo.User interface
type UserRepoAdapter struct {
	inner user.UserRepo
}

func NewUserRepo(gqlClient *gqlclient.Client) repo.User {
	return &UserRepoAdapter{
		inner: gqlClient.UserRepo,
	}
}

func (a *UserRepoAdapter) FindAll(ctx context.Context) ([]*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByID(ctx context.Context, uid id.UserID) (*accountsUser.User, error) {
	return a.inner.FindByID(ctx, uid.String())
}

func (a *UserRepoAdapter) FindByIDs(ctx context.Context, ids id.UserIDList) ([]*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByIDsWithPagination(ctx context.Context, ids id.UserIDList, p *usecasex.Pagination, orders ...string) ([]*accountsUser.User, *usecasex.PageInfo, error) {
	return nil, nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindBySub(ctx context.Context, sub string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByEmail(ctx context.Context, email string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByName(ctx context.Context, name string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByAlias(ctx context.Context, alias string) (*accountsUser.User, error) {
	return a.inner.FindByAlias(ctx, alias)
}

func (a *UserRepoAdapter) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) SearchByKeyword(ctx context.Context, keyword string, orders ...string) ([]*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByVerification(ctx context.Context, code string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindByPasswordResetRequest(ctx context.Context, token string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) FindBySubOrCreate(ctx context.Context, u *accountsUser.User, sub string) (*accountsUser.User, error) {
	return nil, rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) Create(ctx context.Context, u *accountsUser.User) error {
	return rerror.ErrNotImplemented
}

func (a *UserRepoAdapter) Save(ctx context.Context, u *accountsUser.User) error {
	return fmt.Errorf("save user: %w", rerror.ErrNotImplemented)
}

func (a *UserRepoAdapter) Remove(ctx context.Context, uid id.UserID) error {
	return rerror.ErrNotImplemented
}

// NewContainer creates a new repo.Container using GQL client adapters
func NewContainer(gqlClient *gqlclient.Client) *repo.Container {
	return &repo.Container{
		Workspace:   NewWorkspaceRepo(gqlClient),
		User:        NewUserRepo(gqlClient),
		Transaction: &NoopTransaction{},
	}
}

// NoopTransaction is a no-op transaction for GQL client
type NoopTransaction struct{}

func (t *NoopTransaction) Begin(ctx context.Context) (usecasex.Tx, error) {
	return &NoopTx{ctx: ctx}, nil
}

type NoopTx struct {
	ctx context.Context
}

func (t *NoopTx) Context() context.Context {
	return t.ctx
}

func (t *NoopTx) Commit() {
}

func (t *NoopTx) Rollback() {
}

func (t *NoopTx) IsCommitted() bool {
	return false
}

func (t *NoopTx) Done(ctx context.Context) {
}

func (t *NoopTx) End(ctx context.Context) error {
	return nil
}
