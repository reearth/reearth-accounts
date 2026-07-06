// Package workspaceuc holds the admin cross-tenant workspace usecases. V1 is
// read-only (list). The "must be approved" gate is applied by the
// RequireApproved middleware, so these usecases carry no extra authorization.
package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

// ListWorkspacesUseCase lists workspaces across all tenants, optionally filtered
// by a name/alias keyword, with offset pagination.
type ListWorkspacesUseCase struct {
	repo workspace.Repo
}

// NewListWorkspacesUseCase is a Wire provider for ListWorkspacesUseCase.
func NewListWorkspacesUseCase(repo workspace.Repo) *ListWorkspacesUseCase {
	return &ListWorkspacesUseCase{repo: repo}
}

// Execute returns the matching workspaces and pagination info.
func (uc *ListWorkspacesUseCase) Execute(ctx context.Context, keyword *string, pagination *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	return uc.repo.FindAll(ctx, keyword, pagination)
}
