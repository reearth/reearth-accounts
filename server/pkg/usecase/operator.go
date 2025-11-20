package usecase

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/util"
)

type Operator struct {
	User                   *id.UserID
	ReadableWorkspaces     id.WorkspaceIDList
	WritableWorkspaces     id.WorkspaceIDList
	OwningWorkspaces       id.WorkspaceIDList
	MaintainableWorkspaces id.WorkspaceIDList
	DefaultPolicy          *workspace.PolicyID
}

func (o *Operator) Workspaces(r workspace.Role) id.WorkspaceIDList {
	if o == nil {
		return nil
	}
	if r == workspace.RoleReader {
		return o.ReadableWorkspaces
	}
	if r == workspace.RoleWriter {
		return o.WritableWorkspaces
	}
	if r == workspace.RoleMaintainer {
		return o.MaintainableWorkspaces
	}
	if r == workspace.RoleOwner {
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

func (o *Operator) Policy(p *workspace.PolicyID) *workspace.PolicyID {
	if p == nil && o.DefaultPolicy != nil && *o.DefaultPolicy != "" {
		return util.CloneRef(o.DefaultPolicy)
	}
	if p != nil && *p == "" {
		return nil
	}
	return p
}
