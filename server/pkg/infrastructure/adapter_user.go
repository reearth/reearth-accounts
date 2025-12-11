package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	internalRepo "github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

// userAdapter adapts internal repo.User to pkg repo.User interface
type userAdapter struct {
	internal internalRepo.User
}

// NewUserAdapter creates an adapter that bridges internal User implementation to pkg User interface
func NewUserAdapter(internal internalRepo.User) repo.User {
	return &userAdapter{internal: internal}
}

func (a *userAdapter) FindAll(ctx context.Context) (user.List, error) {
	list, err := a.internal.FindAll(ctx)
	return user.List(list), err
}

func (a *userAdapter) FindByID(ctx context.Context, uid id.UserID) (*user.User, error) {
	return a.internal.FindByID(ctx, user.ID(uid))
}

func (a *userAdapter) FindByIDs(ctx context.Context, ids id.UserIDList) (user.List, error) {
	userIDs := make(user.IDList, len(ids))
	for i, uid := range ids {
		userIDs[i] = user.ID(uid)
	}
	list, err := a.internal.FindByIDs(ctx, userIDs)
	return user.List(list), err
}

func (a *userAdapter) FindByIDsWithPagination(ctx context.Context, ids id.UserIDList, pagination *usecasex.Pagination, nameOrAlias ...string) ([]*user.User, *usecasex.PageInfo, error) {
	// Internal implementation doesn't have pagination for FindByIDs
	// We'll implement a simple version here
	users, err := a.FindByIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	// Apply name/alias filter if provided
	if len(nameOrAlias) > 0 && nameOrAlias[0] != "" {
		filtered := make([]*user.User, 0)
		for _, u := range users {
			if u.Name() == nameOrAlias[0] || u.Alias() == nameOrAlias[0] {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	// Apply pagination
	total := int64(len(users))
	start, end := 0, len(users)

	if pagination != nil {
		if pagination.Offset != nil {
			start = int(pagination.Offset.Offset)
			if start > len(users) {
				start = len(users)
			}
			if pagination.Offset.Limit > 0 {
				end = start + int(pagination.Offset.Limit)
				if end > len(users) {
					end = len(users)
				}
			}
		} else if pagination.Cursor != nil {
			// For cursor-based pagination, use First
			if pagination.Cursor.First != nil {
				end = int(*pagination.Cursor.First)
				if end > len(users) {
					end = len(users)
				}
			}
		}
	}

	return users[start:end], &usecasex.PageInfo{
		TotalCount: total,
	}, nil
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

func (a *userAdapter) Remove(ctx context.Context, uid id.UserID) error {
	return a.internal.Remove(ctx, user.ID(uid))
}
