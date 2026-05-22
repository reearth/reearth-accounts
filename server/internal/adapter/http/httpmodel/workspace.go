package httpmodel

import (
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// WorkspaceMemberResponse mirrors WorkspaceUserMember/IntegrationMember (flattened).
type WorkspaceMemberResponse struct {
	UserID        string `json:"user_id,omitempty"`
	IntegrationID string `json:"integration_id,omitempty"`
	Role          string `json:"role"`
}

// WorkspaceMetadataResponse mirrors WorkspaceMetadata.
type WorkspaceMetadataResponse struct {
	Description  string `json:"description"`
	Website      string `json:"website"`
	Location     string `json:"location"`
	BillingEmail string `json:"billing_email"`
	PhotoURL     string `json:"photo_url"`
}

// WorkspaceResponse mirrors the GraphQL Workspace type.
type WorkspaceResponse struct {
	ID       string                     `json:"id"`
	Name     string                     `json:"name"`
	Alias    string                     `json:"alias"`
	Personal bool                       `json:"personal"`
	Members  []WorkspaceMemberResponse  `json:"members"`
	Metadata *WorkspaceMetadataResponse `json:"metadata"`
}

// NewWorkspaceResponse converts a domain workspace (no signed-URL resolution).
func NewWorkspaceResponse(w *workspace.Workspace) *WorkspaceResponse {
	if w == nil {
		return nil
	}
	users := w.Members().Users()
	integrations := w.Members().Integrations()
	members := make([]WorkspaceMemberResponse, 0, len(users)+len(integrations))
	for u, m := range users {
		members = append(members, WorkspaceMemberResponse{UserID: u.String(), Role: RoleString(m.Role)})
	}
	for i, m := range integrations {
		members = append(members, WorkspaceMemberResponse{IntegrationID: i.String(), Role: RoleString(m.Role)})
	}
	md := w.Metadata()
	meta := &WorkspaceMetadataResponse{
		Description:  md.Description(),
		Website:      md.Website(),
		Location:     md.Location(),
		BillingEmail: md.BillingEmail(),
		PhotoURL:     md.PhotoURL(),
	}
	return &WorkspaceResponse{
		ID:       w.ID().String(),
		Name:     w.Name(),
		Alias:    w.Alias(),
		Personal: w.IsPersonal(),
		Members:  members,
		Metadata: meta,
	}
}

// NewWorkspaceResponses converts a list.
func NewWorkspaceResponses(ws workspace.List) []*WorkspaceResponse {
	out := make([]*WorkspaceResponse, 0, len(ws))
	for _, w := range ws {
		out = append(out, NewWorkspaceResponse(w))
	}
	return out
}

// --- Request DTOs ---

// CreateWorkspaceRequest mirrors createWorkspace input.
type CreateWorkspaceRequest struct {
	Alias       string  `json:"alias" validate:"required"`
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description,omitempty"`
}

// UpdateWorkspaceRequest mirrors updateWorkspace input (id from path).
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name,omitempty"`
	Alias       *string `json:"alias,omitempty"`
	Description *string `json:"description,omitempty"`
	Website     *string `json:"website,omitempty"`
	PhotoURL    *string `json:"photo_url,omitempty"`
}

// MemberInputRequest is a single user+role.
type MemberInputRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"required,oneof=reader writer maintainer owner"`
}

// AddMembersRequest mirrors addUsersToWorkspace input.
type AddMembersRequest struct {
	Users []MemberInputRequest `json:"users" validate:"required,min=1,dive"`
}

// UpdateMemberRequest mirrors updateUserOfWorkspace input (ids from path).
type UpdateMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=reader writer maintainer owner"`
}

// RemoveMembersRequest mirrors removeMultipleUsersFromWorkspace input.
type RemoveMembersRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1"`
}

// AddIntegrationRequest mirrors addIntegrationToWorkspace input.
type AddIntegrationRequest struct {
	IntegrationID string `json:"integration_id" validate:"required"`
	Role          string `json:"role" validate:"required,oneof=reader writer maintainer owner"`
}

// UpdateIntegrationRequest mirrors updateIntegrationOfWorkspace input (ids from path).
type UpdateIntegrationRequest struct {
	Role string `json:"role" validate:"required,oneof=reader writer maintainer owner"`
}

// RemoveIntegrationsRequest mirrors removeIntegrationsFromWorkspace input.
type RemoveIntegrationsRequest struct {
	IntegrationIDs []string `json:"integration_ids" validate:"required,min=1"`
}

// TransferOwnershipRequest mirrors transferWorkspaceOwnership input.
type TransferOwnershipRequest struct {
	NewOwnerID string `json:"new_owner_id" validate:"required"`
}

// BuildUserRoleMap converts member inputs to the interactor map.
func (r *AddMembersRequest) BuildUserRoleMap() (map[user.ID]role.RoleType, error) {
	m := make(map[user.ID]role.RoleType, len(r.Users))
	for _, mi := range r.Users {
		uid, err := ParseUserID(mi.UserID)
		if err != nil {
			return nil, err
		}
		m[uid] = ParseRole(mi.Role)
	}
	return m, nil
}
