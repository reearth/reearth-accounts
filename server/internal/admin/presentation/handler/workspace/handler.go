// Package workspace implements the admin cross-tenant workspace endpoints
// (list, detail, members, ownership transfer), behind the RequireApproved
// middleware.
package workspace

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
)

// Handler serves the /workspaces endpoints.
type Handler struct {
	get           *workspaceuc.GetWorkspaceUseCase
	list          *workspaceuc.ListWorkspacesUseCase
	members       *workspaceuc.ListWorkspaceMembersUseCase
	transferOwner *workspaceuc.TransferOwnerUseCase
}

// NewHandler is a Wire provider for the workspace Handler.
func NewHandler(
	get *workspaceuc.GetWorkspaceUseCase,
	list *workspaceuc.ListWorkspacesUseCase,
	members *workspaceuc.ListWorkspaceMembersUseCase,
	transferOwner *workspaceuc.TransferOwnerUseCase,
) *Handler {
	return &Handler{get: get, list: list, members: members, transferOwner: transferOwner}
}
