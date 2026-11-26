package workspace

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/util"
)

type Operator struct {
	User *id.UserID
	// Integration is set, and User is nil, for a service caller (ADR-0002).
	Integration *id.IntegrationID
	// Client is set when a registered client token made the call. It is nil
	// for a user's own token. The auth layer sets it once and nothing clears
	// it, so the call stays marked as a client call for its whole life.
	Client                 *Client
	ReadableWorkspaces     id.WorkspaceIDList
	WritableWorkspaces     id.WorkspaceIDList
	OwningWorkspaces       id.WorkspaceIDList
	MaintainableWorkspaces id.WorkspaceIDList
	DefaultPolicy          *PolicyID
}

// IsService reports whether the operator is a service caller with no user.
func (o *Operator) IsService() bool {
	return o != nil && o.User == nil && o.Integration != nil
}

// ViaClient reports whether a registered client token made the call.
func (o *Operator) ViaClient() bool {
	return o != nil && o.Client != nil
}

// IsUserSelf reports whether a user is calling with their own token.
func (o *Operator) IsUserSelf() bool {
	return o != nil && o.User != nil && o.Client == nil
}

// Subject is derived from User and Integration rather than stored twice.
func (o *Operator) Subject() Subject {
	switch {
	case o == nil:
		return Subject{}
	case o.User != nil:
		return UserSubject(*o.User)
	case o.Integration != nil:
		return IntegrationSubject(*o.Integration)
	default:
		return Subject{}
	}
}

func (o *Operator) Workspaces(r role.RoleType) id.WorkspaceIDList {
	if o == nil {
		return nil
	}
	if r == role.RoleReader {
		return o.ReadableWorkspaces
	}
	if r == role.RoleWriter {
		return o.WritableWorkspaces
	}
	if r == role.RoleMaintainer {
		return o.MaintainableWorkspaces
	}
	if r == role.RoleOwner {
		return o.OwningWorkspaces
	}
	return nil
}

func (o *Operator) AllReadableWorkspaces() id.WorkspaceIDList {
	return append(o.ReadableWorkspaces, o.AllWritableWorkspaces()...)
}

func (o *Operator) AllWritableWorkspaces() id.WorkspaceIDList {
	return append(o.WritableWorkspaces, o.AllMaintainingWorkspaces()...)
}

func (o *Operator) AllMaintainingWorkspaces() id.WorkspaceIDList {
	return append(o.MaintainableWorkspaces, o.AllOwningWorkspaces()...)
}

func (o *Operator) AllOwningWorkspaces() id.WorkspaceIDList {
	return o.OwningWorkspaces
}

func (o *Operator) IsReadableWorkspace(ws ...id.WorkspaceID) bool {
	return o.AllReadableWorkspaces().Intersect(ws).Len() > 0
}

func (o *Operator) IsWritableWorkspace(ws ...id.WorkspaceID) bool {
	return o.AllWritableWorkspaces().Intersect(ws).Len() > 0
}

func (o *Operator) IsMaintainingWorkspace(workspace ...id.WorkspaceID) bool {
	return o.AllMaintainingWorkspaces().Intersect(workspace).Len() > 0
}

func (o *Operator) IsOwningWorkspace(ws ...id.WorkspaceID) bool {
	return o.AllOwningWorkspaces().Intersect(ws).Len() > 0
}

func (o *Operator) AddNewWorkspace(ws id.WorkspaceID) {
	o.OwningWorkspaces = append(o.OwningWorkspaces, ws)
}

func (o *Operator) Policy(p *PolicyID) *PolicyID {
	if p == nil && o.DefaultPolicy != nil && *o.DefaultPolicy != "" {
		return util.CloneRef(o.DefaultPolicy)
	}
	if p != nil && *p == "" {
		return nil
	}
	return p
}
