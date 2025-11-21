package interactor

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearth-accounts/server/pkg/usecase"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

type User struct {
	repos *repo.Container
}

func NewUser(r *repo.Container) interfaces.User {
	return &User{
		repos: r,
	}
}

func (i *User) Fetch(ctx context.Context, ids []id.UserID, operator *usecase.Operator) ([]*user.User, error) {
	return i.repos.User.FindByIDs(ctx, ids)
}

func (i *User) FindByID(ctx context.Context, userID id.UserID, operator *usecase.Operator) (*user.User, error) {
	return i.repos.User.FindByID(ctx, userID)
}

func (i *User) FindByIDs(ctx context.Context, ids id.UserIDList, operator *usecase.Operator) ([]*user.User, error) {
	return i.repos.User.FindByIDs(ctx, ids)
}

func (i *User) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return i.repos.User.FindByEmail(ctx, email)
}

func (i *User) FindByName(ctx context.Context, name string) (*user.User, error) {
	return i.repos.User.FindByName(ctx, name)
}

func (i *User) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return i.repos.User.FindByNameOrEmail(ctx, nameOrEmail)
}

func (i *User) FindByVerification(ctx context.Context, code string) (*user.User, error) {
	return i.repos.User.FindByVerification(ctx, code)
}

func (i *User) Create(ctx context.Context, u *user.User, operator *usecase.Operator) (*user.User, error) {
	if err := i.repos.User.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (i *User) Update(ctx context.Context, u *user.User, operator *usecase.Operator) (*user.User, error) {
	if err := i.repos.User.Save(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (i *User) UpdateProfile(ctx context.Context, userID id.UserID, name, email string, operator *usecase.Operator) (*user.User, error) {
	u, err := i.repos.User.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	u.UpdateName(name)
	if err := u.UpdateEmail(email); err != nil {
		return nil, err
	}

	if err := i.repos.User.Save(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (i *User) Remove(ctx context.Context, userID id.UserID, operator *usecase.Operator) error {
	return i.repos.User.Remove(ctx, userID)
}

func (i *User) SearchUser(ctx context.Context, workspaces id.WorkspaceIDList, keyword string, pagination *usecasex.Pagination) ([]*user.User, *usecasex.PageInfo, error) {
	// Simple implementation - can be enhanced based on requirements
	users, err := i.repos.User.SearchByKeyword(ctx, keyword)
	if err != nil {
		return nil, nil, err
	}

	// TODO: Implement proper pagination
	pageInfo := &usecasex.PageInfo{
		TotalCount: int64(len(users)),
		HasNextPage: false,
		HasPreviousPage: false,
	}

	return users, pageInfo, nil
}
