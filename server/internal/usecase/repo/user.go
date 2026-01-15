package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var ErrDuplicatedUser = rerror.NewE(i18n.T("duplicated user"))

//go:generate mockgen -source=./user.go -destination=./mock_repo/mock_user.go -package mock_repo
type User = user.Repo

type UserQuery = user.Query
