package httpmodel

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// WorkspaceMemberResponse mirrors WorkspaceUserMember/IntegrationMember (flattened).
type WorkspaceMemberResponse struct {
	UserID        string `json:"user_id,omitempty"`
	IntegrationID string `json:"integration_id,omitempty"`
	Role          string `json:"role"`
	// User is set only with ?expand=members.user. Its photo_url is the stored
	// value, and a soft-deleted user appears like a live one, both as in GraphQL.
	User *MemberUserResponse `json:"user,omitempty"`
}

// WorkspaceMetadataResponse mirrors WorkspaceMetadata.
type WorkspaceMetadataResponse struct {
	Description  string `json:"description"`
	Website      string `json:"website"`
	Location     string `json:"location"`
	BillingEmail string `json:"billing_email"`
	// A signed URL that expires, so do not store it.
	PhotoURL string `json:"photo_url"`
}

// WorkspaceResponse mirrors the GraphQL Workspace type.
type WorkspaceResponse struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Alias         string                     `json:"alias"`
	Personal      bool                       `json:"personal"`
	Members       []WorkspaceMemberResponse  `json:"members"`
	Metadata      *WorkspaceMetadataResponse `json:"metadata"`
	ScimAvailable bool                       `json:"scim_available"`
	CreatedAt     *time.Time                 `json:"created_at,omitempty"`
	CreatedBy     *string                    `json:"created_by,omitempty"`
	DeletedAt     *time.Time                 `json:"deleted_at,omitempty"`
}

// MemberUsers says whether a workspace response embeds each member's user,
// and whose email the embedded users show.
type MemberUsers struct {
	Embed  bool
	Viewer EmailViewer
}

// NewWorkspaceResponse converts a domain workspace (no signed-URL resolution),
// dropping user members missing from users, as GraphQL does. billing decides
// who sees the billing email.
func NewWorkspaceResponse(w *workspace.Workspace, users map[user.ID]*user.User, memberUsers MemberUsers, billing EmailViewer) *WorkspaceResponse {
	if w == nil {
		return nil
	}
	memberRoles := w.Members().Users()
	integrations := w.Members().Integrations()
	members := make([]WorkspaceMemberResponse, 0, len(memberRoles)+len(integrations))
	for uid, m := range memberRoles {
		u, ok := users[uid]
		if !ok {
			continue
		}
		mr := WorkspaceMemberResponse{UserID: uid.String(), Role: RoleString(m.Role)}
		if memberUsers.Embed {
			mr.User = NewMemberUserResponse(u, memberUsers.Viewer)
		}
		members = append(members, mr)
	}
	for i, m := range integrations {
		members = append(members, WorkspaceMemberResponse{IntegrationID: i.String(), Role: RoleString(m.Role)})
	}
	md := w.Metadata()
	meta := &WorkspaceMetadataResponse{
		Description:  md.Description(),
		Website:      md.Website(),
		Location:     md.Location(),
		BillingEmail: billing.billingEmail(w),
		PhotoURL:     md.PhotoURL(),
	}
	res := &WorkspaceResponse{
		ID:            w.ID().String(),
		Name:          w.Name(),
		Alias:         w.Alias(),
		Personal:      w.IsPersonal(),
		Members:       members,
		Metadata:      meta,
		ScimAvailable: w.ScimAvailable(),
		CreatedAt:     w.CreatedAt(),
		DeletedAt:     w.DeletedAt(),
	}
	if cb := w.CreatedBy(); cb != nil {
		s := cb.String()
		res.CreatedBy = &s
	}
	return res
}

// billingEmail shows a workspace's billing email only to its members, as
// GraphQL does, unless the route already limits who gets the record.
func (v EmailViewer) billingEmail(w *workspace.Workspace) string {
	if v.all || (v.self != nil && w.Members().HasUser(*v.self)) {
		return w.Metadata().BillingEmail()
	}
	return ""
}

// NewWorkspaceResponses converts a list.
func NewWorkspaceResponses(ws workspace.List, users map[user.ID]*user.User, memberUsers MemberUsers, billing EmailViewer) []*WorkspaceResponse {
	out := make([]*WorkspaceResponse, 0, len(ws))
	for _, w := range ws {
		out = append(out, NewWorkspaceResponse(w, users, memberUsers, billing))
	}
	return out
}

// --- Request DTOs ---

// CreateWorkspaceRequest mirrors createWorkspace input.
type CreateWorkspaceRequest struct {
	Alias       string  `json:"alias" validate:"required"`
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description,omitempty"`
	// SkipOwnerMembership, if true, creates the workspace without joining the
	// caller as owner (for callers that manage membership themselves, e.g.
	// LINKS-Veda). Defaults to false, preserving the normal create-and-own flow.
	// REST-only; not exposed via GraphQL.
	SkipOwnerMembership bool `json:"skip_owner_membership,omitempty"`
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
