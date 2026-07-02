package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

const adminUserColumns = "id, email, name, picture_url, status, approved_by, approved_at, created_at, updated_at"

type AdminUser struct {
	c *Client
}

func NewAdminUser(c *Client) adminuser.Repo { return &AdminUser{c: c} }

func adminUserModel(a gen.AdminUser) (*adminuser.AdminUser, error) {
	return pgdoc.AdminUserRow{
		ID:         a.ID,
		Email:      a.Email,
		Name:       a.Name,
		PictureURL: a.PictureUrl,
		Status:     a.Status,
		ApprovedBy: a.ApprovedBy,
		ApprovedAt: a.ApprovedAt,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
	}.Model()
}

func adminUserModels(as []gen.AdminUser) (adminuser.List, error) {
	out := make(adminuser.List, 0, len(as))
	for _, a := range as {
		m, err := adminUserModel(a)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *AdminUser) FindByEmail(ctx context.Context, email string) (*adminuser.AdminUser, error) {
	row, err := r.c.queries(ctx).AdminUserFindByEmail(ctx, adminuser.NormalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return adminUserModel(row)
}

func (r *AdminUser) FindByID(ctx context.Context, uid adminuser.ID) (*adminuser.AdminUser, error) {
	row, err := r.c.queries(ctx).AdminUserFindByID(ctx, uid.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return adminUserModel(row)
}

func (r *AdminUser) FindByIDs(ctx context.Context, ids adminuser.IDList) (adminuser.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.c.queries(ctx).AdminUserFindByIDs(ctx, ids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := adminUserModels(rows)
	if err != nil {
		return nil, err
	}
	return orderAdminUsersByIDs(ids, list), nil
}

// orderAdminUsersByIDs reorders rows to match the requested id order and drops
// missing ones, matching the Mongo/in-memory FindByIDs behavior.
func orderAdminUsersByIDs(ids adminuser.IDList, rows adminuser.List) adminuser.List {
	m := make(map[adminuser.ID]*adminuser.AdminUser, len(rows))
	for _, r := range rows {
		if r != nil {
			m[r.ID()] = r
		}
	}
	out := make(adminuser.List, 0, len(ids))
	for _, id := range ids {
		if u, ok := m[id]; ok {
			out = append(out, u)
		}
	}
	return out
}

func (r *AdminUser) List(ctx context.Context, f adminuser.ListFilter) (adminuser.List, *usecasex.PageInfo, error) {
	if f.Pagination != nil && f.Pagination.Cursor != nil {
		return nil, nil, adminuser.ErrCursorPaginationUnsupported
	}

	var where []string
	var args []any
	if f.Status != nil {
		args = append(args, f.Status.String())
		where = append(where, "status = $"+itoa(len(args)))
	}
	base := "FROM admin_users"
	if len(where) > 0 {
		base += " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.c.db(ctx).QueryRow(ctx, "SELECT count(*) "+base, args...).Scan(&total); err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}

	q := "SELECT " + adminUserColumns + " " + base + " ORDER BY created_at, id"
	var hasNext, hasPrev bool
	if f.Pagination != nil && f.Pagination.Offset != nil {
		off := f.Pagination.Offset
		q += " LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
		args = append(args, off.Limit, off.Offset)
		hasPrev = off.Offset > 0
		hasNext = off.Offset+off.Limit < total
	}

	rows, err := r.c.db(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanAdminUsers(rows)
	if err != nil {
		return nil, nil, err
	}
	return list, usecasex.NewPageInfo(total, nil, nil, hasNext, hasPrev), nil
}

func (r *AdminUser) Save(ctx context.Context, u *adminuser.AdminUser) error {
	if u == nil {
		return nil
	}
	row := pgdoc.NewAdminUserRow(*u)
	if err := r.c.queries(ctx).AdminUserUpsert(ctx, gen.AdminUserUpsertParams{
		ID:         row.ID,
		Email:      row.Email,
		Name:       row.Name,
		PictureUrl: row.PictureURL,
		Status:     row.Status,
		ApprovedBy: row.ApprovedBy,
		ApprovedAt: row.ApprovedAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}); err != nil {
		if isUniqueViolation(err) {
			return adminuser.ErrDuplicatedAdminUser
		}
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func scanAdminUsers(rows pgx.Rows) (adminuser.List, error) {
	defer rows.Close()
	var out adminuser.List
	for rows.Next() {
		var d pgdoc.AdminUserRow
		if err := rows.Scan(
			&d.ID, &d.Email, &d.Name, &d.PictureURL, &d.Status,
			&d.ApprovedBy, &d.ApprovedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m, err := d.Model()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, rerror.ErrInternalBy(err)
	}
	return out, nil
}
