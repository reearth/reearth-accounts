package memory

import (
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

func New() *repo.Container {
	tx := &usecasex.NopTransaction{}
	return &repo.Container{
		User:        NewUser(),
		Workspace:   NewWorkspace(),
		Role:        NewRole(),
		Permittable: NewPermittable(),
		Transaction: tx,
		Transactor:  repo.TransactorFromTransaction{Tx: tx},
		Config:      NewConfig(),
	}
}
