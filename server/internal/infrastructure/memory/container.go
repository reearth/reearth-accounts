package memory

import (
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

func New() *repo.Container {
	users, workspaces := NewUser(), NewWorkspace()
	workspaces.users = users
	return &repo.Container{
		AdminUser:        NewAdminUser(),
		User:             users,
		Workspace:        workspaces,
		Role:             NewRole(),
		Outbox:           NewOutbox(),
		Permittable:      NewPermittable(),
		PolicyAcceptance: NewPolicyAcceptance(),
		PolicyNotice:     NewPolicyNotice(),
		PolicyVersion:    NewPolicyVersion(),
		Transaction:      &usecasex.NopTransaction{},
		Config:           NewConfig(),
	}
}
