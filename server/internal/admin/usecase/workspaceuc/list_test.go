package workspaceuc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ws(name, alias string) *workspace.Workspace {
	return workspace.New().NewID().Name(name).Alias(alias).MustBuild()
}

func TestList_All(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(ws("Alpha", "alpha"), ws("Beta", "beta"))
	uc := NewListWorkspacesUseCase(repo)

	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)
}

func TestList_ByIDs_ReturnsMatching_OmitsUnknown(t *testing.T) {
	ctx := context.Background()
	alpha := ws("Alpha", "alpha")
	beta := ws("Beta", "beta")
	repo := memory.NewWorkspaceWith(alpha, beta)
	uc := NewListWorkspacesUseCase(repo)

	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{IDs: workspace.IDList{alpha.ID(), workspace.NewID()}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, alpha.ID(), got[0].ID())
	assert.Nil(t, pi)
}

func TestList_Keyword(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(ws("Alpha", "alpha"), ws("Beta", "beta"))
	uc := NewListWorkspacesUseCase(repo)

	kw := "alph"
	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{Keyword: &kw})
	require.NoError(t, err)
	require.Equal(t, 1, len(got))
	assert.Equal(t, "Alpha", got[0].Name())
	assert.Equal(t, int64(1), pi.TotalCount)
}

func TestList_RejectsCursorPagination(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(ws("A", "a"))
	uc := NewListWorkspacesUseCase(repo)

	cur := usecasex.Cursor("x")
	first := int64(1)
	p := usecasex.CursorPagination{First: &first, After: &cur}.Wrap()
	_, _, err := uc.Execute(ctx, ListWorkspacesInput{Pagination: p})
	assert.ErrorIs(t, err, workspace.ErrCursorPaginationUnsupported)
}

func TestList_Pagination(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(ws("A", "a"), ws("B", "b"), ws("C", "c"))
	uc := NewListWorkspacesUseCase(repo)

	p := usecasex.OffsetPagination{Offset: 1, Limit: 1}.Wrap()
	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{Pagination: p})
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, int64(3), pi.TotalCount)
	assert.True(t, pi.HasNextPage)
	assert.True(t, pi.HasPreviousPage)
}
