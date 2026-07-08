package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type User struct {
	c *Client
}

func NewUser(c *Client) user.Repo { return &User{c: c} }

func rowToUserRow(r gen.User) *pgdoc.UserRow {
	return &pgdoc.UserRow{
		ID: r.ID, Name: r.Name, Alias: r.Alias, Email: r.Email, Workspace: r.Workspace,
		Password: r.Password, Subs: r.Subs, LatestLogoutAt: r.LatestLogoutAt,
		Metadata: r.Metadata, Verification: r.Verification, PasswordReset: r.PasswordReset,
		Team: r.Team, Lang: r.Lang, Theme: r.Theme, UpdatedAt: r.UpdatedAt,
	}
}

func userModel(r gen.User) (*user.User, error) { return rowToUserRow(r).Model() }

func userModels(rs []gen.User) (user.List, error) {
	out := make(user.List, 0, len(rs))
	for _, r := range rs {
		m, err := userModel(r)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *User) insertParams(u *user.User) gen.UserUpsertParams {
	d := pgdoc.NewUserRow(u)
	return gen.UserUpsertParams{
		ID: d.ID, Name: d.Name, Alias: d.Alias, Email: d.Email, Workspace: d.Workspace,
		Password: d.Password, Subs: d.Subs, LatestLogoutAt: d.LatestLogoutAt,
		Metadata: d.Metadata, Verification: d.Verification, PasswordReset: d.PasswordReset, UpdatedAt: d.UpdatedAt,
	}
}

func (r *User) Create(ctx context.Context, u *user.User) error {
	d := pgdoc.NewUserRow(u)
	err := r.c.queries(ctx).UserInsert(ctx, gen.UserInsertParams{
		ID: d.ID, Name: d.Name, Alias: d.Alias, Email: d.Email, Workspace: d.Workspace,
		Password: d.Password, Subs: d.Subs, LatestLogoutAt: d.LatestLogoutAt,
		Metadata: d.Metadata, Verification: d.Verification, PasswordReset: d.PasswordReset, UpdatedAt: d.UpdatedAt,
	})
	if isUniqueViolation(err) {
		return user.ErrDuplicatedUser
	}
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *User) Save(ctx context.Context, u *user.User) error {
	if err := r.c.queries(ctx).UserUpsert(ctx, r.insertParams(u)); err != nil {
		if isUniqueViolation(err) {
			return user.ErrDuplicatedUser
		}
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *User) Remove(ctx context.Context, uid id.UserID) error {
	if err := r.c.queries(ctx).UserDelete(ctx, uid.String()); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func one(ctx context.Context, r gen.User, err error) (*user.User, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return userModel(r)
}

func (r *User) FindByID(ctx context.Context, uid id.UserID) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByID(ctx, uid.String())
	return one(ctx, row, err)
}

func (r *User) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.c.queries(ctx).UserFindByIDs(ctx, ids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := userModels(rows)
	if err != nil {
		return nil, err
	}
	// preserve requested-id ordering with nil for missing ids (mongo parity)
	byID := map[string]*user.User{}
	for _, u := range list {
		byID[u.ID().String()] = u
	}
	out := make(user.List, 0, len(ids))
	for _, want := range ids {
		out = append(out, byID[want.String()])
	}
	return out, nil
}

func (r *User) FindAll(ctx context.Context) (user.List, error) {
	rows, err := r.c.queries(ctx).UserFindAll(ctx)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return userModels(rows)
}

func (r *User) FindAllWithPagination(ctx context.Context, keyword *string, p *usecasex.Pagination) (user.List, *usecasex.PageInfo, error) {
	if p != nil && p.Cursor != nil {
		return nil, nil, user.ErrCursorPaginationUnsupported
	}
	var where []string
	var args []any
	if keyword != nil && strings.TrimSpace(*keyword) != "" {
		args = append(args, likeContains(strings.TrimSpace(*keyword)))
		i := itoa(len(args))
		where = append(where, "(name ILIKE $"+i+" OR alias ILIKE $"+i+" OR email ILIKE $"+i+")")
	}
	base := "FROM users"
	if len(where) > 0 {
		base += " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.c.db(ctx).QueryRow(ctx, "SELECT count(*) "+base, args...).Scan(&total); err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}

	q := "SELECT " + userColumns + " " + base + " ORDER BY id"
	var hasNext, hasPrev bool
	if p != nil && p.Offset != nil {
		q += " LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
		args = append(args, p.Offset.Limit, p.Offset.Offset)
		hasPrev = p.Offset.Offset > 0
		hasNext = p.Offset.Offset+p.Offset.Limit < total
	}
	rows, err := r.c.db(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanUsers(rows)
	if err != nil {
		return nil, nil, err
	}
	return list, usecasex.NewPageInfo(total, cur(list, true), cur(list, false), hasNext, hasPrev), nil
}

func (r *User) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByEmail(ctx, email)
	return one(ctx, row, err)
}

func (r *User) FindByName(ctx context.Context, name string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByName(ctx, name)
	return one(ctx, row, err)
}

func (r *User) FindByAlias(ctx context.Context, alias string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByAlias(ctx, alias)
	return one(ctx, row, err)
}

func (r *User) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindBySub(ctx, sub)
	return one(ctx, row, err)
}

func (r *User) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByNameOrEmail(ctx, nameOrEmail)
	return one(ctx, row, err)
}

func (r *User) FindByVerification(ctx context.Context, code string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByVerification(ctx, code)
	return one(ctx, row, err)
}

func (r *User) FindByPasswordResetRequest(ctx context.Context, token string) (*user.User, error) {
	row, err := r.c.queries(ctx).UserFindByPasswordResetRequest(ctx, token)
	return one(ctx, row, err)
}

// FindByNameOrAlias does a case-insensitive substring match for mongo parity.
func (r *User) FindByNameOrAlias(ctx context.Context, nameOrAlias string) (user.List, error) {
	kw := likeContains(nameOrAlias)
	rows, err := r.c.db(ctx).Query(ctx,
		`SELECT `+userColumns+` FROM users WHERE name ILIKE $1 OR alias ILIKE $1 ORDER BY name LIMIT 50`, kw)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanUsers(rows)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return list, nil
}

// SearchByKeyword requires >=3 chars and caps at 10 results for mongo parity.
func (r *User) SearchByKeyword(ctx context.Context, keyword string) (user.List, error) {
	if len(keyword) < 3 {
		return nil, nil
	}
	kw := likeContains(keyword)
	rows, err := r.c.db(ctx).Query(ctx,
		`SELECT `+userColumns+` FROM users WHERE name ILIKE $1 OR email ILIKE $1 ORDER BY name LIMIT 10`, kw)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanUsers(rows)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return list, nil
}

func (r *User) FindBySubOrCreate(ctx context.Context, u *user.User, sub string) (*user.User, error) {
	if existing, err := r.FindBySub(ctx, sub); err == nil {
		return existing, nil
	} else if !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	if err := r.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *User) FindByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, p *usecasex.Pagination) (user.List, *usecasex.PageInfo, error) {
	return paginateUsers(ctx, r.c, ids.Strings(), alias, p)
}

func likeContains(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return "%" + s + "%"
}

// userColumns matches scanUsers/gen.User scan order; avoid SELECT * to keep scanning stable.
const userColumns = "id, name, alias, email, workspace, password, subs, " +
	"latest_logout_at, metadata, verification, password_reset, team, lang, theme, updated_at"

func scanUsers(rows pgx.Rows) (user.List, error) {
	defer rows.Close()
	out := user.List{}
	for rows.Next() {
		var g gen.User
		if err := rows.Scan(
			&g.ID, &g.Name, &g.Alias, &g.Email, &g.Workspace, &g.Password, &g.Subs,
			&g.LatestLogoutAt, &g.Metadata, &g.Verification, &g.PasswordReset,
			&g.Team, &g.Lang, &g.Theme, &g.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m, err := userModel(g)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func cur(list user.List, start bool) *usecasex.Cursor {
	if len(list) == 0 {
		return nil
	}
	var s string
	if start {
		s = list[0].ID().String()
	} else {
		s = list[len(list)-1].ID().String()
	}
	c := usecasex.Cursor(s)
	return &c
}

func paginateUsers(ctx context.Context, c *Client, ids []string, alias *string, p *usecasex.Pagination) (user.List, *usecasex.PageInfo, error) {
	where := []string{"id = ANY($1::text[])"}
	args := []any{ids}
	if alias != nil && *alias != "" {
		args = append(args, likeContains(*alias))
		where = append(where, "(name ILIKE $"+itoa(len(args))+" OR alias ILIKE $"+itoa(len(args))+")")
	}
	base := "FROM users WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := c.db(ctx).QueryRow(ctx, "SELECT count(*) "+base, args...).Scan(&total); err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}

	if p != nil && p.Cursor != nil {
		return cursorPageUsers(ctx, c, base, args, total, p.Cursor)
	}
	if p != nil && p.Offset != nil {
		q := "SELECT " + userColumns + " " + base + " ORDER BY id LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
		args = append(args, p.Offset.Limit, p.Offset.Offset)
		rows, err := c.db(ctx).Query(ctx, q, args...)
		if err != nil {
			return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		list, err := scanUsers(rows)
		if err != nil {
			return nil, nil, err
		}
		hasPrev := p.Offset.Offset > 0
		hasNext := p.Offset.Offset+p.Offset.Limit < total
		return list, usecasex.NewPageInfo(total, cur(list, true), cur(list, false), hasNext, hasPrev), nil
	}

	rows, err := c.db(ctx).Query(ctx, "SELECT "+userColumns+" "+base+" ORDER BY id", args...)
	if err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanUsers(rows)
	if err != nil {
		return nil, nil, err
	}
	return list, usecasex.NewPageInfo(total, cur(list, true), cur(list, false), false, false), nil
}

func cursorPageUsers(ctx context.Context, c *Client, base string, args []any, total int64, cp *usecasex.CursorPagination) (user.List, *usecasex.PageInfo, error) {
	forward := cp.First != nil
	var limit int64
	conds := base
	if forward {
		limit = *cp.First
		if cp.After != nil {
			args = append(args, string(*cp.After))
			conds += " AND id > $" + itoa(len(args))
		}
		conds += " ORDER BY id ASC LIMIT $" + itoa(len(args)+1)
	} else {
		if cp.Last != nil {
			limit = *cp.Last
		}
		if cp.Before != nil {
			args = append(args, string(*cp.Before))
			conds += " AND id < $" + itoa(len(args))
		}
		conds += " ORDER BY id DESC LIMIT $" + itoa(len(args)+1)
	}
	args = append(args, limit+1) // +1 to detect hasMore
	rows, err := c.db(ctx).Query(ctx, "SELECT "+userColumns+" "+conds, args...)
	if err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := scanUsers(rows)
	if err != nil {
		return nil, nil, err
	}

	hasMore := int64(len(list)) > limit
	if hasMore {
		list = list[:limit]
	}
	if !forward { // queried DESC; reverse to ascending id order
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	hasNext := (forward && hasMore) || (!forward && cp.Before != nil)
	hasPrev := (!forward && hasMore) || (forward && cp.After != nil)
	return list, usecasex.NewPageInfo(total, cur(list, true), cur(list, false), hasNext, hasPrev), nil
}
