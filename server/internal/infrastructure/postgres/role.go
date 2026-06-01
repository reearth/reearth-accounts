package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/rerror"
)

type Role struct {
	c *Client
}

func NewRole(c *Client) role.Repo { return &Role{c: c} }

func roleModel(r gen.Role) (*role.Role, error) {
	return pgdoc.RoleRow{ID: r.ID, Name: r.Name}.Model()
}

func roleModels(rs []gen.Role) (role.List, error) {
	out := make(role.List, 0, len(rs))
	for _, r := range rs {
		m, err := roleModel(r)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Role) FindAll(ctx context.Context) (role.List, error) {
	rows, err := r.c.queries(ctx).RoleFindAll(ctx)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return roleModels(rows)
}

func (r *Role) FindByID(ctx context.Context, rid id.RoleID) (*role.Role, error) {
	row, err := r.c.queries(ctx).RoleFindByID(ctx, rid.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return roleModel(row)
}

// FindByIDs returns found rows without preserving request order (mongo parity).
func (r *Role) FindByIDs(ctx context.Context, ids id.RoleIDList) (role.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.c.queries(ctx).RoleFindByIDs(ctx, ids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return roleModels(rows)
}

func (r *Role) FindByName(ctx context.Context, name string) (*role.Role, error) {
	row, err := r.c.queries(ctx).RoleFindByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return roleModel(row)
}

func (r *Role) Save(ctx context.Context, rl role.Role) error {
	row := pgdoc.NewRoleRow(rl)
	if err := r.c.queries(ctx).RoleUpsert(ctx, gen.RoleUpsertParams{ID: row.ID, Name: row.Name}); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Role) Remove(ctx context.Context, rid id.RoleID) error {
	if err := r.c.queries(ctx).RoleDelete(ctx, rid.String()); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}
