package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type Permittable struct {
	c *Client
}

func NewPermittable(c *Client) permittable.Repo { return &Permittable{c: c} }

func (r *Permittable) hydrate(ctx context.Context, rows []gen.Permittable) (permittable.List, error) {
	if len(rows) == 0 {
		return permittable.List{}, nil
	}
	ids := make([]string, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.ID)
	}
	wrRows, err := r.c.queries(ctx).PermittableWorkspaceRolesByPermittableIDs(ctx, ids)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	wrByPerm := map[string][]pgdoc.PermittableWorkspaceRoleRow{}
	for _, wr := range wrRows {
		wrByPerm[wr.PermittableID] = append(wrByPerm[wr.PermittableID], pgdoc.PermittableWorkspaceRoleRow{
			PermittableID: wr.PermittableID, WorkspaceID: wr.WorkspaceID, RoleID: wr.RoleID,
		})
	}
	out := make(permittable.List, 0, len(rows))
	for _, p := range rows {
		row := pgdoc.PermittableRow{ID: p.ID, UserID: p.UserID, RoleIDs: p.RoleIds, UpdatedAt: p.UpdatedAt}
		m, err := pgdoc.PermittableModel(row, wrByPerm[p.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Permittable) FindByUserID(ctx context.Context, uid user.ID) (*permittable.Permittable, error) {
	row, err := r.c.queries(ctx).PermittableFindByUserID(ctx, uid.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := r.hydrate(ctx, []gen.Permittable{row})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

// FindByUserIDs returns ErrNotFound on empty result for mongo parity.
func (r *Permittable) FindByUserIDs(ctx context.Context, ids user.IDList) (permittable.List, error) {
	rows, err := r.c.queries(ctx).PermittableFindByUserIDs(ctx, ids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := r.hydrate(ctx, rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, rerror.ErrNotFound
	}
	return list, nil
}

// FindByRoleID returns ErrNotFound on empty result for mongo parity.
func (r *Permittable) FindByRoleID(ctx context.Context, rid id.RoleID) (permittable.List, error) {
	rows, err := r.c.queries(ctx).PermittableFindByRoleID(ctx, rid.String())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := r.hydrate(ctx, rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, rerror.ErrNotFound
	}
	return list, nil
}

func (r *Permittable) Save(ctx context.Context, p permittable.Permittable) error {
	row, wrs := pgdoc.NewPermittableRow(p)
	return r.c.WithinTransaction(ctx, func(ctx context.Context) error {
		q := r.c.queries(ctx)
		// ON CONFLICT (user_id) keeps the existing row's id; use the returned id for child rows.
		pid, err := q.PermittableUpsert(ctx, gen.PermittableUpsertParams{
			ID: row.ID, UserID: row.UserID, RoleIds: row.RoleIDs, UpdatedAt: row.UpdatedAt,
		})
		if err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		if err := q.PermittableWorkspaceRolesDeleteByPermittable(ctx, pid); err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		for _, wr := range wrs {
			if err := q.PermittableWorkspaceRoleInsert(ctx, gen.PermittableWorkspaceRoleInsertParams{
				PermittableID: pid, WorkspaceID: wr.WorkspaceID, RoleID: wr.RoleID,
			}); err != nil {
				return rerror.ErrInternalByWithContext(ctx, err)
			}
		}
		return nil
	})
}

func (r *Permittable) SaveMany(ctx context.Context, list permittable.List) error {
	if len(list) == 0 {
		return nil
	}
	return r.c.WithinTransaction(ctx, func(ctx context.Context) error {
		for _, p := range list {
			if p == nil {
				continue
			}
			if err := r.Save(ctx, *p); err != nil {
				return err
			}
		}
		return nil
	})
}
