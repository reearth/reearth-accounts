package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearthx/rerror"
)

type Outbox struct {
	c *Client
}

func NewOutbox(c *Client) outbox.Repo { return &Outbox{c: c} }

func (r *Outbox) ClaimDue(ctx context.Context, now time.Time, c outbox.Claim) (*outbox.Record, error) {
	row, err := r.c.queries(ctx).OutboxClaimDue(ctx, gen.OutboxClaimDueParams{Token: &c.Token, Until: &c.Until, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return outboxModel(ctx, row)
}

func (r *Outbox) Complete(ctx context.Context, k outbox.Key, token string, t outbox.Transition, now time.Time) (bool, error) {
	n, err := r.c.queries(ctx).OutboxComplete(ctx, gen.OutboxCompleteParams{
		Key:           string(k),
		NextAttemptAt: t.NextAttemptAt,
		Now:           now,
		Reason:        string(t.Reason),
		State:         string(t.To),
		Token:         &token,
	})
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return n == 1, nil
}

func (r *Outbox) CountByNotice(ctx context.Context, n id.PolicyNoticeID) (map[outbox.State]int, error) {
	rows, err := r.c.queries(ctx).OutboxCountByNotice(ctx, n.String())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	counts := outbox.EmptyCounts()
	for _, row := range rows {
		counts[outbox.State(row.State)] = int(row.N)
	}
	return counts, nil
}

func (r *Outbox) Enqueue(ctx context.Context, rs []outbox.Record) error {
	if len(rs) == 0 {
		return nil
	}
	var p gen.OutboxEnqueueParams
	for _, rec := range rs {
		subject, err := json.Marshal(rec.Subject)
		if err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		uid, address := "", ""
		if u, ok := rec.Recipient.User(); ok {
			uid = u.String()
		} else {
			address, _ = rec.Recipient.Address()
		}
		p.Addresses = append(p.Addresses, address)
		p.Attempts = append(p.Attempts, int32(rec.Attempts))
		p.CreatedAts = append(p.CreatedAts, rec.CreatedAt)
		p.Keys = append(p.Keys, string(rec.Key))
		p.Kinds = append(p.Kinds, string(rec.Kind))
		p.NextAttemptAts = append(p.NextAttemptAts, rec.NextAttemptAt)
		p.Reasons = append(p.Reasons, string(rec.Reason))
		p.States = append(p.States, string(rec.State))
		p.Subjects = append(p.Subjects, subject)
		p.UpdatedAts = append(p.UpdatedAts, rec.UpdatedAt)
		p.UserIds = append(p.UserIds, uid)
	}
	if err := r.c.queries(ctx).OutboxEnqueue(ctx, p); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Outbox) FindByKey(ctx context.Context, k outbox.Key) (*outbox.Record, error) {
	row, err := r.c.queries(ctx).OutboxFindByKey(ctx, string(k))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return outboxModel(ctx, row)
}

func (r *Outbox) RemoveByUser(ctx context.Context, u id.UserID) error {
	uid := u.String()
	if err := r.c.queries(ctx).OutboxRemoveByUser(ctx, &uid); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Outbox) Sweep(ctx context.Context, now time.Time, ttl time.Duration) (outbox.SweepResult, error) {
	var res outbox.SweepResult
	q := r.c.queries(ctx)
	expired, err := q.OutboxExpireClaims(ctx, &now)
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.Expired = int(expired)
	cutoff := now.Add(-ttl)
	timedOut, err := q.OutboxFailStale(ctx, gen.OutboxFailStaleParams{Now: &now, Cutoff: cutoff})
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.TimedOut = int(timedOut)
	deleted, err := q.OutboxDeleteRetired(ctx, &cutoff)
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.Deleted = int(deleted)
	return res, nil
}

func outboxModel(ctx context.Context, row gen.MailOutbox) (*outbox.Record, error) {
	rec := &outbox.Record{
		Attempts:      int(row.Attempts),
		CreatedAt:     row.CreatedAt.UTC(),
		Key:           outbox.Key(row.Key),
		Kind:          mail.Kind(row.Kind),
		NextAttemptAt: row.NextAttemptAt.UTC(),
		Reason:        outbox.Reason(row.Reason),
		State:         outbox.State(row.State),
		UpdatedAt:     row.UpdatedAt.UTC(),
	}
	if err := json.Unmarshal(row.Subject, &rec.Subject); err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	if row.ClaimToken != nil && row.ClaimUntil != nil {
		rec.Claim = &outbox.Claim{Token: *row.ClaimToken, Until: row.ClaimUntil.UTC()}
	}
	if row.EndedAt != nil {
		e := row.EndedAt.UTC()
		rec.EndedAt = &e
	}
	switch {
	case row.RecipientUserID != nil:
		u, err := id.UserIDFrom(*row.RecipientUserID)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		rec.Recipient = outbox.ToUser(u)
	case row.RecipientAddress != nil:
		rec.Recipient = outbox.ToAddress(*row.RecipientAddress)
	}
	return rec, nil
}
