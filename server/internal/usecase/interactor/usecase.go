package interactor

import (
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type uc struct {
	readableWorkspaces     id.WorkspaceIDList
	writableWorkspaces     id.WorkspaceIDList
	maintainableWorkspaces id.WorkspaceIDList
	ownableWorkspaces      id.WorkspaceIDList
}

func Usecase() *uc {
	return &uc{}
}

func (u *uc) WithReadableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.readableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithWritableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.writableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithMaintainableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.maintainableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithOwnableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.ownableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) CheckPermission(op *workspace.Operator) error {
	ok := true
	if op != nil {
		if u.readableWorkspaces != nil {
			ok = op.IsReadableWorkspace(u.readableWorkspaces...)
		}
		if ok && u.writableWorkspaces != nil {
			ok = op.IsWritableWorkspace(u.writableWorkspaces...)
		}
		if ok && u.maintainableWorkspaces != nil {
			ok = op.IsMaintainingWorkspace(u.maintainableWorkspaces...)
		}
		if ok && u.ownableWorkspaces != nil {
			ok = op.IsOwningWorkspace(u.ownableWorkspaces...)
		}
	}
	if !ok {
		return interfaces.ErrOperationDenied
	}
	return nil
}
