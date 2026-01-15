package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var ErrDuplicateWorkspaceAlias = rerror.NewE(i18n.T("duplicate workspace alias"))

//go:generate mockgen -source=./workspace.go -destination=./mock_repo/mock_workspace.go -package mock_repo
type Workspace = workspace.Repo
