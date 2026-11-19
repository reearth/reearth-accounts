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

type versionJSON struct {
	EffectiveAt time.Time         `json:"effectiveAt"`
	Required    bool              `json:"required"`
	Title       map[string]string `json:"title"`
	URL         map[string]string `json:"url,omitempty"`
	Version     string            `json:"version"`
}

type withdrawalJSON struct {
	At      time.Time `json:"at"`
	Version string    `json:"version"`
}

func (r *PolicyVersion) ArchiveVersions(ctx context.Context, h legal.TypeHead) error {
	for _, v := range []*legal.VersionRecord{h.Current, h.Upcoming} {
		if v == nil {
			continue
		}
		title, err := json.Marshal(nonNil(v.Title))
		if err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		url, err := json.Marshal(nonNil(v.URL))
		if err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
		if err := r.c.queries(ctx).PolicyVersionArchive(ctx, gen.PolicyVersionArchiveParams{
			Type:        string(v.Type),
			Version:     v.Version.String(),
			EffectiveAt: v.EffectiveAt,
			Required:    v.Required,
			Title:       title,
			Url:         url,
			Revision:    h.Revision,
		}); err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
	}
	return nil
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
	current, err := marshalVersion(next.Current)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	upcoming, err := marshalVersion(next.Upcoming)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}

	q := r.c.queries(ctx)
	var n int64
	if next.Revision == 1 {
		n, err = q.PolicyTypeInsert(ctx, gen.PolicyTypeInsertParams{
			Type: string(next.Type), Revision: next.Revision, AppliedAt: next.AppliedAt,
			AppliedVersion: next.Applied.Version.String(), AppliedDigest: next.Applied.Digest,
			Current: current, Upcoming: upcoming, Withdrawn: wj,
		})
	} else {
		n, err = q.PolicyTypeSwap(ctx, gen.PolicyTypeSwapParams{
			Type: string(next.Type), Revision: next.Revision, AppliedAt: next.AppliedAt,
			AppliedVersion: next.Applied.Version.String(), AppliedDigest: next.Applied.Digest,
			Current: current, Upcoming: upcoming, Withdrawn: wj,
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

func marshalVersion(v *legal.VersionRecord) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(versionJSON{
		EffectiveAt: v.EffectiveAt,
		Required:    v.Required,
		Title:       v.Title,
		URL:         v.URL,
		Version:     v.Version.String(),
	})
}

func nonNil(t legal.LocalizedText) legal.LocalizedText {
	if t == nil {
		return legal.LocalizedText{}
	}
	return t
}

func policyTypeModel(row gen.PolicyType) (legal.TypeHead, error) {
	t := legal.Type(row.Type)
	applied, err := legal.ParseVersion(row.AppliedVersion)
	if err != nil {
		return legal.TypeHead{}, err
	}
	h := legal.TypeHead{
		Applied:   legal.Applied{Digest: row.AppliedDigest, Version: applied},
		AppliedAt: row.AppliedAt.UTC(),
		Revision:  row.Revision,
		Type:      t,
	}
	if h.Current, err = unmarshalVersion(t, row.Current); err != nil {
		return legal.TypeHead{}, err
	}
	if h.Upcoming, err = unmarshalVersion(t, row.Upcoming); err != nil {
		return legal.TypeHead{}, err
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

func unmarshalVersion(t legal.Type, b []byte) (*legal.VersionRecord, error) {
	if b == nil {
		return nil, nil
	}
	var j versionJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	v, err := legal.ParseVersion(j.Version)
	if err != nil {
		return nil, err
	}
	return &legal.VersionRecord{
		Ref:         legal.Ref{Type: t, Version: v},
		EffectiveAt: j.EffectiveAt.UTC(),
		Required:    j.Required,
		Title:       j.Title,
		URL:         j.URL,
	}, nil
}
