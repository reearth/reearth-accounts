package workspace

import (
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
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
// role. Name and Email are always present in the response; they are empty
// strings when the member's underlying user cannot be resolved.
type WorkspaceMemberResponse struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`  // empty string if the member's user cannot be resolved
	Email    string `json:"email"` // empty string if the member's user cannot be resolved
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

// TransferredWorkspaceResponse is the workspace a transfer was applied to.
type TransferredWorkspaceResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Alias    string `json:"alias"`
	Personal bool   `json:"personal"`
} // @name TransferredWorkspace

// TransferWorkspaceOwnerResponse is the result of a committed ownership
// transfer. DemotedUserIDs is never null; it is empty when the workspace had
// no owner other than the target.
type TransferWorkspaceOwnerResponse struct {
	Workspace      TransferredWorkspaceResponse `json:"workspace"`
	NewOwner       WorkspaceMemberResponse      `json:"newOwner"`
	DemotedUserIDs []string                     `json:"demotedUserIds"`
} // @name TransferWorkspaceOwnerResponse

func newTransferWorkspaceOwnerResponse(out *workspaceuc.TransferOwnerOutput) TransferWorkspaceOwnerResponse {
	ws := out.Workspace
	res := TransferWorkspaceOwnerResponse{
		Workspace: TransferredWorkspaceResponse{
			ID:       ws.ID().String(),
			Name:     ws.Name(),
			Alias:    ws.Alias(),
			Personal: ws.IsPersonal(),
		},
		NewOwner: WorkspaceMemberResponse{
			UserID: out.NewOwner.ID().String(),
			Name:   out.NewOwner.Name(),
			Email:  out.NewOwner.Email(),
		},
		DemotedUserIDs: make([]string, 0, len(out.DemotedUserIDs)),
	}
	if m := ws.Members().User(out.NewOwner.ID()); m != nil {
		res.NewOwner.Role = string(m.Role)
		res.NewOwner.Disabled = m.Disabled
	}
	for _, uid := range out.DemotedUserIDs {
		res.DemotedUserIDs = append(res.DemotedUserIDs, uid.String())
	}
	return res
}
