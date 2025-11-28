package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/usecase"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

type Workspace interface {
	Fetch(context.Context, []id.WorkspaceID, *usecase.Operator) ([]*workspace.Workspace, error)
	FindByID(context.Context, id.WorkspaceID, *usecase.Operator) (*workspace.Workspace, error)
	FindByIDs(context.Context, id.WorkspaceIDList, *usecase.Operator) ([]*workspace.Workspace, error)
	FindByUser(context.Context, id.UserID, *usecase.Operator) ([]*workspace.Workspace, error)
	Create(context.Context, string, id.UserID, *usecase.Operator) (*workspace.Workspace, error)
	Update(context.Context, id.WorkspaceID, string, *usecase.Operator) (*workspace.Workspace, error)
	Remove(context.Context, id.WorkspaceID, *usecase.Operator) error
	AddMember(context.Context, id.WorkspaceID, id.UserID, workspace.Role, *usecase.Operator) (*workspace.Workspace, error)
	RemoveMember(context.Context, id.WorkspaceID, id.UserID, *usecase.Operator) (*workspace.Workspace, error)
	UpdateMember(context.Context, id.WorkspaceID, id.UserID, workspace.Role, *usecase.Operator) (*workspace.Workspace, error)
	FindByUserWithPagination(context.Context, id.UserID, *usecasex.Pagination, *usecase.Operator) ([]*workspace.Workspace, *usecasex.PageInfo, error)
}
