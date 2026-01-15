package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/role"
)

//go:generate mockgen -source=./role.go -destination=./mock_repo/mock_role.go -package mock_repo
type Role = role.Repo
