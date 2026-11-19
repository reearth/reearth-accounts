package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
)

type PolicyNotice struct {
	c *Client
}

func NewPolicyNotice(c *Client) legal.NoticeRepo { return &PolicyNotice{c: c} }

func (r *PolicyNotice) AdvanceFanout(ctx context.Context, n id.PolicyNoticeID, lease legal.Lease, cursor string, end legal.FanoutEnd) (bool, error) {
	rows, err := r.c.queries(ctx).PolicyNoticeAdvanceFanout(ctx, gen.PolicyNoticeAdvanceFanoutParams{
		Cursor: cursor, FanoutEnd: string(end), ID: n.String(), Token: lease.Token, Until: lease.Until,
	})
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return rows == 1, nil
}

func (r *PolicyNotice) ClaimFanout(ctx context.Context, now time.Time, lease legal.Lease) (*legal.Notice, error) {
	row, err := r.c.queries(ctx).PolicyNoticeClaimFanout(ctx, gen.PolicyNoticeClaimFanoutParams{Now: now, Token: lease.Token, Until: lease.Until})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	n, err := noticeModel(row)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return &n, nil
}

func (r *PolicyNotice) Create(ctx context.Context, n legal.Notice) (legal.Notice, legal.CreateOutcome, error) {
	q := r.c.queries(ctx)
	row, err := q.PolicyNoticeCreate(ctx, gen.PolicyNoticeCreateParams{
		Audience:    string(n.Audience),
		Bump:        string(n.Bump),
		CreatedAt:   n.CreatedAt,
		ID:          n.ID.String(),
		RequestedBy: n.RequestedBy.String(),
		Type:        string(n.Ref.Type),
		Version:     n.Ref.Version.String(),
	})
	outcome := legal.NoticeCreated
	if errors.Is(err, pgx.ErrNoRows) {
		outcome = legal.NoticeReopened
		row, err = q.PolicyNoticeReopen(ctx, gen.PolicyNoticeReopenParams{
			Audience: string(n.Audience), Type: string(n.Ref.Type), Version: n.Ref.Version.String(),
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		outcome = legal.NoticeExisting
		row, err = q.PolicyNoticeFindByVersionAudience(ctx, gen.PolicyNoticeFindByVersionAudienceParams{
			Audience: string(n.Audience), Type: string(n.Ref.Type), Version: n.Ref.Version.String(),
		})
	}
	if err != nil {
		return legal.Notice{}, "", rerror.ErrInternalByWithContext(ctx, err)
	}
	stored, err := noticeModel(row)
	if err != nil {
		return legal.Notice{}, "", rerror.ErrInternalByWithContext(ctx, err)
	}
	return stored, outcome, nil
}

func (r *PolicyNotice) FindByRef(ctx context.Context, ref legal.Ref) ([]legal.Notice, error) {
	rows, err := r.c.queries(ctx).PolicyNoticeFindByRef(ctx, gen.PolicyNoticeFindByRefParams{Type: string(ref.Type), Version: ref.Version.String()})
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.Notice, 0, len(rows))
	for _, row := range rows {
		n, err := noticeModel(row)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, n)
	}
	return res, nil
}

func (r *PolicyNotice) ReleaseFanout(ctx context.Context, n id.PolicyNoticeID, token string) error {
	if err := r.c.queries(ctx).PolicyNoticeReleaseFanout(ctx, gen.PolicyNoticeReleaseFanoutParams{ID: n.String(), Token: token}); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func noticeModel(row gen.PolicyNotice) (legal.Notice, error) {
	nid, err := id.PolicyNoticeIDFrom(row.ID)
	if err != nil {
		return legal.Notice{}, err
	}
	by, err := id.AdminUserIDFrom(row.RequestedBy)
	if err != nil {
		return legal.Notice{}, err
	}
	t, err := legal.ParseType(row.Type)
	if err != nil {
		return legal.Notice{}, err
	}
	v, err := legal.ParseVersion(row.Version)
	if err != nil {
		return legal.Notice{}, err
	}
	n := legal.Notice{
		Audience:    legal.Audience(row.Audience),
		Bump:        legal.Bump(row.Bump),
		CreatedAt:   row.CreatedAt.UTC(),
		Fanout:      legal.Fanout{Cursor: row.FanoutCursor, End: legal.FanoutEnd(row.FanoutEnd)},
		ID:          nid,
		Ref:         legal.Ref{Type: t, Version: v},
		RequestedBy: by,
	}
	if row.LeaseToken != nil && row.LeaseUntil != nil {
		n.Fanout.Lease = &legal.Lease{Token: *row.LeaseToken, Until: row.LeaseUntil.UTC()}
	}
	return n, nil
}
