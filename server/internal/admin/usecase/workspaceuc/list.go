// Package workspaceuc holds the admin cross-tenant workspace usecases. V1 is
// read-only (list). The "must be approved" gate is applied by the
// RequireApproved middleware, so these usecases carry no extra authorization.
package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

// ListWorkspacesUseCase lists admin workspaces. See ListWorkspacesInput for its
// two modes (keyword listing vs. batch-by-IDs).
type ListWorkspacesUseCase struct {
	repo workspace.Repo
}

// NewListWorkspacesUseCase is a Wire provider for ListWorkspacesUseCase.
func NewListWorkspacesUseCase(repo workspace.Repo) *ListWorkspacesUseCase {
	return &ListWorkspacesUseCase{repo: repo}
}

// ListWorkspacesInput selects which workspaces to return. When IDs is non-empty
// it is a batch-by-IDs lookup (unknown IDs omitted) and Keyword/Pagination are
// ignored; otherwise it is a keyword-filtered, offset-paginated listing. Both
// modes read across all tenants (the admin repo is unfiltered).
type ListWorkspacesInput struct {
	Keyword    *string
	Pagination *usecasex.Pagination
	IDs        workspace.IDList
}

// Execute returns the matching workspaces. Batch-by-IDs mode returns a nil
// *PageInfo; callers derive counts from the list.
func (uc *ListWorkspacesUseCase) Execute(ctx context.Context, in ListWorkspacesInput) (workspace.List, *usecasex.PageInfo, error) {
	if len(in.IDs) > 0 {
		list, err := uc.repo.FindByIDs(ctx, in.IDs)
		if err != nil {
			return nil, nil, err
		}
		return list, nil, nil
	}
	return uc.repo.FindAll(ctx, in.Keyword, in.Pagination)
}
