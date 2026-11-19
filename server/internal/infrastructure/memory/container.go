package memory

import (
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

func New() *repo.Container {
	return &repo.Container{
		AdminUser:        NewAdminUser(),
		User:             NewUser(),
		Workspace:        NewWorkspace(),
		Role:             NewRole(),
		Permittable:      NewPermittable(),
		PolicyAcceptance: NewPolicyAcceptance(),
		PolicyVersion:    NewPolicyVersion(),
		Transaction:      &usecasex.NopTransaction{},
		Config:           NewConfig(),
	}
}
