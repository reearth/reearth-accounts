package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
)

type PolicyVersion struct {
	c *Client
}

func NewPolicyVersion(c *Client) legal.VersionRepo { return &PolicyVersion{c: c} }

type withdrawalJSON struct {
	At      time.Time `json:"at"`
	Version string    `json:"version"`
}

func (r *PolicyVersion) CompareAndSwapHead(ctx context.Context, next legal.TypeHead) (bool, error) {
	withdrawn := make([]withdrawalJSON, 0, len(next.Withdrawn))
	for _, w := range next.Withdrawn {
		withdrawn = append(withdrawn, withdrawalJSON{At: w.At, Version: w.Version.String()})
	}
	wj, err := json.Marshal(withdrawn)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	var upVersion *string
	var upAt *time.Time
	if u := next.Upcoming; u != nil {
		v := u.Version.String()
		upVersion, upAt = &v, &u.EffectiveAt
	}

	q := r.c.queries(ctx)
	var n int64
	if next.Revision == 1 {
		n, err = q.PolicyTypeInsert(ctx, gen.PolicyTypeInsertParams{
			Type: string(next.Type), Revision: next.Revision, AppliedAt: next.AppliedAt,
			UpcomingVersion: upVersion, UpcomingEffectiveAt: upAt, Withdrawn: wj,
		})
	} else {
		n, err = q.PolicyTypeSwap(ctx, gen.PolicyTypeSwapParams{
			Type: string(next.Type), Revision: next.Revision, AppliedAt: next.AppliedAt,
			UpcomingVersion: upVersion, UpcomingEffectiveAt: upAt, Withdrawn: wj,
		})
	}
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return n == 1, nil
}

func (r *PolicyVersion) FindHead(ctx context.Context, t legal.Type) (legal.TypeHead, error) {
	row, err := r.c.queries(ctx).PolicyTypeFind(ctx, string(t))
	if errors.Is(err, pgx.ErrNoRows) {
		return legal.TypeHead{Type: t}, nil
	}
	if err != nil {
		return legal.TypeHead{}, rerror.ErrInternalByWithContext(ctx, err)
	}
	h, err := policyTypeModel(row)
	if err != nil {
		return legal.TypeHead{}, rerror.ErrInternalByWithContext(ctx, err)
	}
	return h, nil
}

func (r *PolicyVersion) FindHeads(ctx context.Context) ([]legal.TypeHead, error) {
	rows, err := r.c.queries(ctx).PolicyTypeFindAll(ctx)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.TypeHead, 0, len(rows))
	for _, row := range rows {
		h, err := policyTypeModel(row)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, h)
	}
	return res, nil
}

func (r *PolicyVersion) FindVersions(ctx context.Context) ([]legal.VersionRecord, error) {
	rows, err := r.c.queries(ctx).PolicyVersionFindAll(ctx)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.VersionRecord, 0, len(rows))
	for _, row := range rows {
		v, err := policyVersionModel(row)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, v)
	}
	return res, nil
}

func (r *PolicyVersion) UpsertVersion(ctx context.Context, v legal.VersionRecord, now time.Time) error {
	title, err := json.Marshal(nonNil(v.Title))
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	url, err := json.Marshal(nonNil(v.URL))
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	if err := r.c.queries(ctx).PolicyVersionUpsert(ctx, gen.PolicyVersionUpsertParams{
		Type:        string(v.Type),
		Version:     v.Version.String(),
		EffectiveAt: v.EffectiveAt,
		Required:    v.Required,
		Title:       title,
		Url:         url,
		Stamp:       v.Stamp,
		Now:         now,
	}); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func nonNil(t legal.LocalizedText) legal.LocalizedText {
	if t == nil {
		return legal.LocalizedText{}
	}
	return t
}

func policyTypeModel(row gen.PolicyType) (legal.TypeHead, error) {
	h := legal.TypeHead{AppliedAt: row.AppliedAt.UTC(), Revision: row.Revision, Type: legal.Type(row.Type)}
	if row.UpcomingVersion != nil && row.UpcomingEffectiveAt != nil {
		v, err := legal.ParseVersion(*row.UpcomingVersion)
		if err != nil {
			return legal.TypeHead{}, err
		}
		h.Upcoming = &legal.Scheduled{EffectiveAt: row.UpcomingEffectiveAt.UTC(), Version: v}
	}
	var withdrawn []withdrawalJSON
	if err := json.Unmarshal(row.Withdrawn, &withdrawn); err != nil {
		return legal.TypeHead{}, err
	}
	for _, w := range withdrawn {
		v, err := legal.ParseVersion(w.Version)
		if err != nil {
			return legal.TypeHead{}, err
		}
		h.Withdrawn = append(h.Withdrawn, legal.Withdrawal{At: w.At.UTC(), Version: v})
	}
	return h, nil
}

func policyVersionModel(row gen.PolicyVersion) (legal.VersionRecord, error) {
	v, err := legal.ParseVersion(row.Version)
	if err != nil {
		return legal.VersionRecord{}, err
	}
	rec := legal.VersionRecord{
		Ref:         legal.Ref{Type: legal.Type(row.Type), Version: v},
		EffectiveAt: row.EffectiveAt.UTC(),
		Required:    row.Required,
		Stamp:       row.Stamp.UTC(),
	}
	if err := json.Unmarshal(row.Title, &rec.Title); err != nil {
		return legal.VersionRecord{}, err
	}
	if err := json.Unmarshal(row.Url, &rec.URL); err != nil {
		return legal.VersionRecord{}, err
	}
	if len(rec.URL) == 0 {
		rec.URL = nil
	}
	return rec, nil
}
