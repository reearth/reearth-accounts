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
// it is a batch-by-IDs lookup (unknown IDs omitted) and Keyword/Personal/Pagination
// are ignored; otherwise it is a keyword-filtered, offset-paginated listing. Both
// modes read across all tenants (the admin repo is unfiltered).
//
// Personal filters by workspace type: nil returns both types, true returns only
// personal workspaces, false returns only team (non-personal) workspaces.
type ListWorkspacesInput struct {
	Keyword    *string
	Personal   *bool
	Pagination *usecasex.Pagination
	IDs        workspace.IDList
}

// Execute returns the matching workspaces. Batch-by-IDs mode returns a nil
// *PageInfo; callers derive counts from the list.
func (uc *ListWorkspacesUseCase) Execute(ctx context.Context, in ListWorkspacesInput) (workspace.List, *usecasex.PageInfo, error) {
	if len(in.IDs) == 0 {
		// StatusAll preserves this usecase's existing behavior: active and
		// soft-deleted workspaces mixed together, unfiltered by status.
		return uc.repo.FindAll(ctx, in.Keyword, in.Personal, workspace.StatusAll, in.Pagination, false)
	}

	list, err := uc.repo.FindByIDs(ctx, in.IDs)
	if err != nil {
		return nil, nil, err
	}

	// FindByIDs order is backend-dependent (Mongo matches the requested order,
	// the memory repo sorts by ID). Re-order to match in.IDs so the API response
	// order is stable across backends; unknown IDs are naturally omitted. Each
	// workspace is emitted at most once, so a repeated input ID does not produce
	// a duplicate entry even though de-duplication is normally done by the caller.
	byID := make(map[workspace.ID]*workspace.Workspace, len(list))
	for _, w := range list {
		byID[w.ID()] = w
	}
	ordered := make(workspace.List, 0, len(in.IDs))
	seen := make(map[workspace.ID]struct{}, len(in.IDs))
	for _, wid := range in.IDs {
		if _, dup := seen[wid]; dup {
			continue
		}
		if w, ok := byID[wid]; ok {
			seen[wid] = struct{}{}
			ordered = append(ordered, w)
		}
	}
	return ordered, nil, nil
}
