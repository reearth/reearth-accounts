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

func personalWs(name, alias string) *workspace.Workspace {
	return workspace.New().NewID().Name(name).Alias(alias).Personal(true).MustBuild()
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

func TestList_ByIDs_PreservesRequestedOrder(t *testing.T) {
	ctx := context.Background()
	a := ws("Alpha", "alpha")
	b := ws("Beta", "beta")
	c := ws("Gamma", "gamma")
	repo := memory.NewWorkspaceWith(a, b, c)
	uc := NewListWorkspacesUseCase(repo)

	// Request in an order that does not match the backend's natural (by-ID)
	// order; the response must follow the requested order.
	want := workspace.IDList{c.ID(), a.ID(), b.ID()}
	got, _, err := uc.Execute(ctx, ListWorkspacesInput{IDs: want})
	require.NoError(t, err)
	require.Len(t, got, 3)
	gotIDs := workspace.IDList{got[0].ID(), got[1].ID(), got[2].ID()}
	assert.Equal(t, want, gotIDs)
}

func TestList_ByIDs_DeduplicatesRepeatedIDs(t *testing.T) {
	ctx := context.Background()
	a := ws("Alpha", "alpha")
	b := ws("Beta", "beta")
	repo := memory.NewWorkspaceWith(a, b)
	uc := NewListWorkspacesUseCase(repo)

	// A repeated input ID must yield the workspace exactly once.
	got, _, err := uc.Execute(ctx, ListWorkspacesInput{IDs: workspace.IDList{a.ID(), a.ID(), b.ID()}})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, a.ID(), got[0].ID())
	assert.Equal(t, b.ID(), got[1].ID())
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

func TestList_PersonalOnly(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(personalWs("Solo", "solo"), ws("Team", "team"))
	uc := NewListWorkspacesUseCase(repo)

	yes := true
	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{Personal: &yes})
	require.NoError(t, err)
	require.Equal(t, 1, len(got))
	assert.Equal(t, "Solo", got[0].Name())
	assert.True(t, got[0].IsPersonal())
	assert.Equal(t, int64(1), pi.TotalCount)
}

func TestList_TeamOnly(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(personalWs("Solo", "solo"), ws("Team", "team"))
	uc := NewListWorkspacesUseCase(repo)

	no := false
	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{Personal: &no})
	require.NoError(t, err)
	require.Equal(t, 1, len(got))
	assert.Equal(t, "Team", got[0].Name())
	assert.False(t, got[0].IsPersonal())
	assert.Equal(t, int64(1), pi.TotalCount)
}

func TestList_AllTypes_WhenPersonalNil(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWorkspaceWith(personalWs("Solo", "solo"), ws("Team", "team"))
	uc := NewListWorkspacesUseCase(repo)

	got, pi, err := uc.Execute(ctx, ListWorkspacesInput{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)
}

func TestList_ByIDs_IgnoresPersonalFilter(t *testing.T) {
	ctx := context.Background()
	solo := personalWs("Solo", "solo")
	team := ws("Team", "team")
	repo := memory.NewWorkspaceWith(solo, team)
	uc := NewListWorkspacesUseCase(repo)

	// Personal is ignored in batch-by-IDs mode: a team workspace resolves even
	// though Personal=true would exclude it in the keyword-listing path.
	yes := true
	got, _, err := uc.Execute(ctx, ListWorkspacesInput{IDs: workspace.IDList{team.ID()}, Personal: &yes})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, team.ID(), got[0].ID())
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
