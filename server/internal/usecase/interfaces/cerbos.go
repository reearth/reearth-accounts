package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
)

const (
	ResourceWorkspace = "workspace"
	ResourceUser      = "user"
	ResourceProject   = "project"
)

const (
	RoleSelf = "self"
)

type CheckPermissionParam struct {
	Service        string
	Resource       string
	Action         string
	WorkspaceAlias string
}

type CheckPermissionResult struct {
	Allowed bool
}

type Cerbos interface {
	CheckPermission(ctx context.Context, userId user.ID, param CheckPermissionParam) (*CheckPermissionResult, error)
}
