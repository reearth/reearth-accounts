package postgres

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Workspace struct {
	c *Client
	f workspace.WorkspaceFilter
}

func NewWorkspace(c *Client) workspace.Repo { return &Workspace{c: c} }

func (r *Workspace) Filtered(f workspace.WorkspaceFilter) workspace.Repo {
	return &Workspace{c: r.c, f: r.f.Merge(f)}
}

// FindAll is not implemented for the PostgreSQL backend: the admin app (which
// is the only consumer of this cross-tenant listing) runs on MongoDB, so the
// Postgres admin list path is intentionally left unimplemented for now.
func (r *Workspace) FindAll(_ context.Context, _ *string, _ *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	return nil, nil, workspace.ErrNotImplemented
}

func (r *Workspace) hydrate(ctx context.Context, rows []gen.Workspace) (workspace.List, error) {
	if len(rows) == 0 {
		return workspace.List{}, nil
	}
	ids := make([]string, 0, len(rows))
	for _, w := range rows {
		ids = append(ids, w.ID)
	}
	q := r.c.queries(ctx)
	memRows, err := q.WorkspaceMembersByWorkspaceIDs(ctx, ids)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	intRows, err := q.WorkspaceIntegrationsByWorkspaceIDs(ctx, ids)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	memByWS := map[string][]pgdoc.WorkspaceMemberRow{}
	for _, m := range memRows {
		memByWS[m.WorkspaceID] = append(memByWS[m.WorkspaceID], pgdoc.WorkspaceMemberRow{
			WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: m.Role, InvitedBy: m.InvitedBy, Disabled: m.Disabled,
		})
	}
	intByWS := map[string][]pgdoc.WorkspaceIntegrationRow{}
	for _, m := range intRows {
		intByWS[m.WorkspaceID] = append(intByWS[m.WorkspaceID], pgdoc.WorkspaceIntegrationRow{
			WorkspaceID: m.WorkspaceID, IntegrationID: m.IntegrationID, Role: m.Role, InvitedBy: m.InvitedBy, Disabled: m.Disabled,
		})
	}
	out := make(workspace.List, 0, len(rows))
	for _, w := range rows {
		row := &pgdoc.WorkspaceRow{
			ID: w.ID, Name: w.Name, Alias: w.Alias, Email: w.Email, Personal: w.Personal,
			Policy: w.Policy, MembersHash: w.MembersHash, Metadata: w.Metadata, UpdatedAt: w.UpdatedAt,
		}
		m, err := pgdoc.WorkspaceModel(row, memByWS[w.ID], intByWS[w.ID])
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Workspace) one(ctx context.Context, w gen.Workspace, err error) (*workspace.Workspace, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := r.hydrate(ctx, []gen.Workspace{w})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, rerror.ErrNotFound
	}
	return list[0], nil
}

// FindByID does not apply the read filter (mongo parity).
func (r *Workspace) FindByID(ctx context.Context, wid id.WorkspaceID) (*workspace.Workspace, error) {
	w, err := r.c.queries(ctx).WorkspaceFindByID(ctx, wid.String())
	return r.one(ctx, w, err)
}

func (r *Workspace) FindByIDs(ctx context.Context, ids id.WorkspaceIDList) (workspace.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	for _, wid := range ids {
		if !r.f.CanRead(wid) {
			return nil, rerror.ErrNotFound
		}
	}
	rows, err := r.c.queries(ctx).WorkspaceFindByIDs(ctx, ids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	list, err := r.hydrate(ctx, rows)
	if err != nil {
		return nil, err
	}
	return list.FilterByID(ids...), nil
}

func (r *Workspace) FindByName(ctx context.Context, name string) (*workspace.Workspace, error) {
	if name == "" {
		return nil, rerror.ErrNotFound
	}
	w, err := r.c.queries(ctx).WorkspaceFindByName(ctx, name)
	ws, err := r.one(ctx, w, err)
	if err != nil {
		return nil, err
	}
	if !r.f.CanRead(ws.ID()) {
		return nil, rerror.ErrNotFound
	}
	return ws, nil
}

func (r *Workspace) FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	if alias == "" {
		return nil, rerror.ErrNotFound
	}
	w, err := r.c.queries(ctx).WorkspaceFindByAlias(ctx, alias)
	return r.one(ctx, w, err)
}

// FindByAliases lowercases inputs because the SQL query compares lower(alias)
// to match the case-insensitive alias unique index.
func (r *Workspace) FindByAliases(ctx context.Context, aliases []string) (workspace.List, error) {
	if len(aliases) == 0 {
		return nil, nil
	}
	lowered := make([]string, len(aliases))
	for i, a := range aliases {
		lowered[i] = strings.ToLower(a)
	}
	rows, err := r.c.queries(ctx).WorkspaceFindByAliases(ctx, lowered)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return r.hydrate(ctx, rows)
}

func (r *Workspace) FindByUser(ctx context.Context, uid user.ID) (workspace.List, error) {
	wsIDs, err := r.c.queries(ctx).WorkspaceIDsByUser(ctx, uid.String())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return r.findByIDStrings(ctx, wsIDs)
}

func (r *Workspace) FindByIntegration(ctx context.Context, iid id.IntegrationID) (workspace.List, error) {
	wsIDs, err := r.c.queries(ctx).WorkspaceIDsByIntegration(ctx, iid.String())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return r.findByIDStrings(ctx, wsIDs)
}

func (r *Workspace) FindByIntegrations(ctx context.Context, iids id.IntegrationIDList) (workspace.List, error) {
	if len(iids) == 0 {
		return nil, nil
	}
	wsIDs, err := r.c.queries(ctx).WorkspaceIDsByIntegrations(ctx, iids.Strings())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return r.findByIDStrings(ctx, wsIDs)
}

func (r *Workspace) findByIDStrings(ctx context.Context, ids []string) (workspace.List, error) {
	if len(ids) == 0 {
		return workspace.List{}, nil
	}
	rows, err := r.c.queries(ctx).WorkspaceFindByIDs(ctx, ids)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return r.hydrate(ctx, rows)
}

func (r *Workspace) FindByUserWithPagination(ctx context.Context, uid user.ID, p *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	wsIDs, err := r.c.queries(ctx).WorkspaceIDsByUser(ctx, uid.String())
	if err != nil {
		return nil, nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return paginateWorkspaces(ctx, r, wsIDs, p)
}

func (r *Workspace) Create(ctx context.Context, ws *workspace.Workspace) error {
	return r.save(ctx, ws)
}

func (r *Workspace) Save(ctx context.Context, ws *workspace.Workspace) error {
	if !r.f.CanWrite(ws.ID()) {
		return repo.ErrOperationDenied
	}
	return r.save(ctx, ws)
}

func (r *Workspace) save(ctx context.Context, ws *workspace.Workspace) error {
	row, members, integrations := pgdoc.NewWorkspaceRows(ws)
	return r.c.WithinTransaction(ctx, func(ctx context.Context) error {
		q := r.c.queries(ctx)
		if err := q.WorkspaceUpsert(ctx, gen.WorkspaceUpsertParams{
			ID: row.ID, Name: row.Name, Alias: row.Alias, Email: row.Email, Personal: row.Personal,
			Policy: row.Policy, MembersHash: row.MembersHash, Metadata: row.Metadata, UpdatedAt: row.UpdatedAt,
		}); err != nil {
			if isUniqueViolation(err) {
				return workspace.ErrDuplicateWorkspaceAlias
			}
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		if err := q.WorkspaceMembersDeleteByWorkspace(ctx, row.ID); err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		for _, m := range members {
			if err := q.WorkspaceMemberInsert(ctx, gen.WorkspaceMemberInsertParams{
				WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: m.Role, InvitedBy: m.InvitedBy, Disabled: m.Disabled,
			}); err != nil {
				return rerror.ErrInternalByWithContext(ctx, err)
			}
		}
		if err := q.WorkspaceIntegrationsDeleteByWorkspace(ctx, row.ID); err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		for _, m := range integrations {
			if err := q.WorkspaceIntegrationInsert(ctx, gen.WorkspaceIntegrationInsertParams{
				WorkspaceID: m.WorkspaceID, IntegrationID: m.IntegrationID, Role: m.Role, InvitedBy: m.InvitedBy, Disabled: m.Disabled,
			}); err != nil {
				return rerror.ErrInternalByWithContext(ctx, err)
			}
		}
		return nil
	})
}

func (r *Workspace) SaveAll(ctx context.Context, list workspace.List) error {
	if len(list) == 0 {
		return nil
	}
	for _, ws := range list {
		if ws != nil && !r.f.CanWrite(ws.ID()) {
			return repo.ErrOperationDenied
		}
	}
	return r.c.WithinTransaction(ctx, func(ctx context.Context) error {
		for _, ws := range list {
			if ws == nil {
				continue
			}
			if err := r.save(ctx, ws); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Workspace) Remove(ctx context.Context, wid id.WorkspaceID) error {
	if !r.f.CanWrite(wid) {
		return repo.ErrOperationDenied
	}
	if err := r.c.queries(ctx).WorkspaceDelete(ctx, wid.String()); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Workspace) RemoveAll(ctx context.Context, ids id.WorkspaceIDList) error {
	if len(ids) == 0 {
		return nil
	}
	for _, wid := range ids {
		if !r.f.CanWrite(wid) {
			return repo.ErrOperationDenied
		}
	}
	return r.c.WithinTransaction(ctx, func(ctx context.Context) error {
		for _, wid := range ids {
			if err := r.c.queries(ctx).WorkspaceDelete(ctx, wid.String()); err != nil {
				return rerror.ErrInternalByWithContext(ctx, err)
			}
		}
		return nil
	})
}

func paginateWorkspaces(ctx context.Context, r *Workspace, ids []string, p *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	total := int64(len(ids))
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)

	page := sorted
	hasNext, hasPrev := false, false
	if p != nil && p.Cursor != nil {
		page, hasNext, hasPrev = sliceCursor(sorted, p.Cursor)
	} else if p != nil && p.Offset != nil {
		page, hasNext, hasPrev = sliceOffset(sorted, p.Offset, total)
	}

	list, err := r.findByIDStrings(ctx, page)
	if err != nil {
		return nil, nil, err
	}

	var startCur, endCur *usecasex.Cursor
	if len(page) > 0 {
		s := usecasex.Cursor(page[0])
		e := usecasex.Cursor(page[len(page)-1])
		startCur, endCur = &s, &e
	}
	return list, usecasex.NewPageInfo(total, startCur, endCur, hasNext, hasPrev), nil
}

func sliceOffset(ids []string, o *usecasex.OffsetPagination, total int64) (page []string, hasNext, hasPrev bool) {
	hasPrev = o.Offset > 0
	hasNext = o.Offset+o.Limit < total
	if o.Offset >= int64(len(ids)) {
		return nil, hasNext, hasPrev
	}
	end := o.Offset + o.Limit
	if end > int64(len(ids)) {
		end = int64(len(ids))
	}
	return ids[o.Offset:end], hasNext, hasPrev
}

func sliceCursor(ids []string, cp *usecasex.CursorPagination) (page []string, hasNext, hasPrev bool) {
	if cp.First != nil { // forward
		start := 0
		if cp.After != nil {
			start = len(ids)
			for i, v := range ids {
				if v > string(*cp.After) {
					start = i
					break
				}
			}
		}
		rest := ids[start:]
		if int64(len(rest)) > *cp.First {
			hasNext = true
			rest = rest[:*cp.First]
		}
		return rest, hasNext, start > 0
	}
	if cp.Last != nil { // backward
		end := len(ids)
		if cp.Before != nil {
			end = 0
			for i := len(ids) - 1; i >= 0; i-- {
				if ids[i] < string(*cp.Before) {
					end = i + 1
					break
				}
			}
		}
		rest := ids[:end]
		if int64(len(rest)) > *cp.Last {
			hasPrev = true
			rest = rest[int64(len(rest))-*cp.Last:]
		}
		return rest, end < len(ids), hasPrev
	}
	return ids, false, false
}
