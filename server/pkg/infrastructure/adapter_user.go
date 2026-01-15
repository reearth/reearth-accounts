package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	internalRepo "github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

// userAdapter adapts internal repo.User to pkg user.Repo interface
type userAdapter struct {
	internal internalRepo.User
}

// NewUserAdapter creates an adapter that bridges internal User implementation to pkg User interface
func NewUserAdapter(internal internalRepo.User) user.Repo {
	return &userAdapter{internal: internal}
}

func (a *userAdapter) FindAll(ctx context.Context) (user.List, error) {
	list, err := a.internal.FindAll(ctx)
	return user.List(list), err
}

func (a *userAdapter) FindByID(ctx context.Context, uid user.ID) (*user.User, error) {
	return a.internal.FindByID(ctx, uid)
}

func (a *userAdapter) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	list, err := a.internal.FindByIDs(ctx, ids)
	return user.List(list), err
}

func (a *userAdapter) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	return a.internal.FindBySub(ctx, sub)
}

func (a *userAdapter) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return a.internal.FindByEmail(ctx, email)
}

func (a *userAdapter) FindByName(ctx context.Context, name string) (*user.User, error) {
	return a.internal.FindByName(ctx, name)
}

func (a *userAdapter) FindByAlias(ctx context.Context, alias string) (*user.User, error) {
	return a.internal.FindByAlias(ctx, alias)
}

func (a *userAdapter) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return a.internal.FindByNameOrEmail(ctx, nameOrEmail)
}

func (a *userAdapter) SearchByKeyword(ctx context.Context, keyword string) (user.List, error) {
	list, err := a.internal.SearchByKeyword(ctx, keyword)
	return user.List(list), err
}

func (a *userAdapter) FindByVerification(ctx context.Context, code string) (*user.User, error) {
	return a.internal.FindByVerification(ctx, code)
}

func (a *userAdapter) FindByPasswordResetRequest(ctx context.Context, token string) (*user.User, error) {
	return a.internal.FindByPasswordResetRequest(ctx, token)
}

func (a *userAdapter) FindBySubOrCreate(ctx context.Context, u *user.User, sub string) (*user.User, error) {
	return a.internal.FindBySubOrCreate(ctx, u, sub)
}

func (a *userAdapter) Create(ctx context.Context, u *user.User) error {
	return a.internal.Create(ctx, u)
}

func (a *userAdapter) Save(ctx context.Context, u *user.User) error {
	return a.internal.Save(ctx, u)
}

func (a *userAdapter) Remove(ctx context.Context, uid user.ID) error {
	return a.internal.Remove(ctx, uid)
}
