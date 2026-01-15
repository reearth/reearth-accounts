package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
)

//go:generate mockgen -source=./permittable.go -destination=./mock_repo/mock_permittable.go -package mock_repo
type Permittable = permittable.Repo
