package workspace

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// WorkspaceResponse is a single workspace in the admin API list.
type WorkspaceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Personal    bool      `json:"personal"`
	MemberCount int       `json:"memberCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
} // @name Workspace

// ListWorkspacesResponse is the paginated list of workspaces.
type ListWorkspacesResponse struct {
	Items      []WorkspaceResponse `json:"items"`
	TotalCount int64               `json:"totalCount"`
	Page       int64               `json:"page"`
	PerPage    int64               `json:"perPage"`
} // @name ListWorkspacesResponse

// WorkspaceMemberResponse is a single member of a workspace, with the member's
// role and (when resolvable) the underlying user's name and email.
type WorkspaceMemberResponse struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
} // @name WorkspaceMember

func newWorkspaceResponse(w *workspace.Workspace) WorkspaceResponse {
	res := WorkspaceResponse{
		ID:        w.ID().String(),
		Name:      w.Name(),
		Alias:     w.Alias(),
		Personal:  w.IsPersonal(),
		UpdatedAt: w.UpdatedAt(),
	}
	if m := w.Members(); m != nil {
		res.MemberCount = m.Count()
	}
	return res
}

func newWorkspaceResponses(list workspace.List) []WorkspaceResponse {
	items := make([]WorkspaceResponse, 0, len(list))
	for _, w := range list {
		items = append(items, newWorkspaceResponse(w))
	}
	return items
}
