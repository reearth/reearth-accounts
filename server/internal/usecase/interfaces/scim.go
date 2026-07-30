package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrSCIMNotEnabled         = rerror.NewE(i18n.T("SCIM is not enabled for this workspace"))
	ErrSCIMTokenAlreadyIssued = rerror.NewE(i18n.T("rotate the token to replace it"))
	ErrSCIMUserNotFound       = rerror.NewE(i18n.T("SCIM user not found"))
)

type ProvisionScimUserParam struct {
	Email       string
	ExternalID  string
	Name        string
	Role        role.RoleType
	WorkspaceID workspace.ID
}

type ScimGroupMember struct {
	ExternalID string
	UserID     *user.ID
}

type Scim interface {
	DeprovisionScimUser(ctx context.Context, workspaceID workspace.ID, externalID string) error
	GenerateScimToken(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (string, error)
	GetScimConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (*workspace.ScimConfig, error)
	GetScimUser(ctx context.Context, workspaceID workspace.ID, userID user.ID) (*user.User, error)
	ListScimUsers(ctx context.Context, workspaceID workspace.ID, filter string) ([]*user.User, error)
	ProvisionScimUser(ctx context.Context, param ProvisionScimUserParam) (*user.User, error)
	SyncScimGroup(ctx context.Context, workspaceID workspace.ID, groupID, groupName string, members []ScimGroupMember) error
	UpdateScimConfig(ctx context.Context, workspaceID workspace.ID, enabled bool, groupRoleMapping map[string]role.RoleType, operator *workspace.Operator) (*workspace.ScimConfig, error)
}
