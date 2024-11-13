package memory

import (
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

func New() *repo.Container {
	return &repo.Container{
		Role:        NewRole(),
		Permittable: NewPermittable(),
		Transaction: &usecasex.NopTransaction{},
	}
}
