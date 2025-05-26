package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/user"
)

//go:generate mockgen -source=./user.go -destination=./mock_repo/mock_user.go -package mock_repo
type User interface {
	FindByID(ctx context.Context, id user.ID) (*user.User, error)
	Save(ctx context.Context, user *user.User) error
}
