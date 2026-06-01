package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// New builds a repo.Container backed by PostgreSQL.
func New(_ context.Context, pool *pgxpool.Pool, users []user.Repo) (*repo.Container, error) {
	c := NewClient(pool)
	return &repo.Container{
		User:        NewUser(c),
		Workspace:   NewWorkspace(c),
		Role:        NewRole(c),
		Permittable: NewPermittable(c),
		Transactor:  c, // *Client.WithinTransaction satisfies repo.Transactor
		Users:       users,
		Config:      NewConfig(pool),
	}, nil
}
