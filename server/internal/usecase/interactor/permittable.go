package interactor

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type Permittable struct {
	permittableRepo permittable.Repo
}

func NewPermittable(r *repo.Container) interfaces.Permittable {
	return &Permittable{
		permittableRepo: r.Permittable,
	}
}

func (i *Permittable) FindByUserIDs(ctx context.Context, userIDs user.IDList) (permittable.List, error) {
	return i.permittableRepo.FindByUserIDs(ctx, userIDs)
}
