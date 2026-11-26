package httpmodel

import (
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// IntegrationWorkspaceResponse is one workspace the acting integration belongs to.
type IntegrationWorkspaceResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Alias string `json:"alias"`
	Role  string `json:"role"`
}

// IntegrationWorkspacesResponse lists the acting integration's workspaces.
type IntegrationWorkspacesResponse struct {
	IntegrationID string                         `json:"integration_id"`
	Workspaces    []IntegrationWorkspaceResponse `json:"workspaces"`
}

// LeaveAllWorkspacesResponse lists the workspaces a leave-all call removed the
// acting integration from, and those it could not.
type LeaveAllWorkspacesResponse struct {
	RemovedWorkspaceIDs []string `json:"removed_workspace_ids"`
	FailedWorkspaceIDs  []string `json:"failed_workspace_ids"`
}

func NewIntegrationWorkspacesResponse(iid workspace.IntegrationID, ms []interfaces.IntegrationMembership) IntegrationWorkspacesResponse {
	res := IntegrationWorkspacesResponse{IntegrationID: iid.String(), Workspaces: make([]IntegrationWorkspaceResponse, 0, len(ms))}
	for _, m := range ms {
		res.Workspaces = append(res.Workspaces, IntegrationWorkspaceResponse{
			ID:    m.Workspace.ID().String(),
			Name:  m.Workspace.Name(),
			Alias: m.Workspace.Alias(),
			Role:  RoleString(m.Role),
		})
	}
	return res
}

func NewLeaveAllWorkspacesResponse(r interfaces.LeaveAllResult) LeaveAllWorkspacesResponse {
	return LeaveAllWorkspacesResponse{
		RemovedWorkspaceIDs: append([]string{}, r.Removed.Strings()...),
		FailedWorkspaceIDs:  append([]string{}, r.Failed.Strings()...),
	}
}
