package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	policyTypesCollection    = "policy_types"
	policyVersionsCollection = "policy_versions"
)

type PolicyVersion struct {
	types    *mongo.Collection
	versions *mongo.Collection
}

func NewPolicyVersion(db *mongo.Database) *PolicyVersion {
	return &PolicyVersion{
		types:    db.Collection(policyTypesCollection),
		versions: db.Collection(policyVersionsCollection),
	}
}

type policyTypeDoc struct {
	Applied   appliedDoc   `bson:"applied"`
	AppliedAt time.Time    `bson:"appliedAt"`
	Revision  int64        `bson:"revision"`
	Type      string       `bson:"_id"`
	Versions  []versionDoc `bson:"versions"`
}

type appliedDoc struct {
	Digest  []byte `bson:"digest"`
	Version string `bson:"version"`
}

type versionDoc struct {
	EffectiveAt time.Time         `bson:"effectiveAt"`
	Required    bool              `bson:"required"`
	Title       map[string]string `bson:"title"`
	URL         map[string]string `bson:"url"`
	Version     string            `bson:"version"`
	WithdrawnAt *time.Time        `bson:"withdrawnAt,omitempty"`
}

type policyVersionDoc struct {
	EffectiveAt time.Time         `bson:"effectiveAt"`
	Required    bool              `bson:"required"`
	Revision    int64             `bson:"revision"`
	Title       map[string]string `bson:"title"`
	Type        string            `bson:"type"`
	URL         map[string]string `bson:"url"`
	Version     string            `bson:"version"`
}

// ArchiveVersions upserts each record only over one with a revision not
// above h.Revision. When a record exists that does not match, the upsert's
// insert hits the unique (type, version) index. Two upserts of a missing
// record race the same way, so a duplicate key retries as a plain update:
// the record now exists, and a non-matching one is newer and kept.
func (r *PolicyVersion) ArchiveVersions(ctx context.Context, h legal.TypeHead) error {
	for _, v := range h.SortedVersions() {
		doc := policyVersionDoc{
			EffectiveAt: v.EffectiveAt,
			Required:    v.Required,
			Revision:    h.Revision,
			Title:       v.Title,
			Type:        string(v.Type),
			URL:         v.URL,
			Version:     v.Version.String(),
		}
		filter := bson.M{"type": doc.Type, "version": doc.Version, "revision": bson.M{"$lte": h.Revision}}
		_, err := r.versions.UpdateOne(ctx, filter, bson.M{"$set": doc}, options.Update().SetUpsert(true))
		if mongo.IsDuplicateKeyError(err) {
			_, err = r.versions.UpdateOne(ctx, filter, bson.M{"$set": doc})
		}
		if err != nil {
			return rerror.ErrInternalByWithContext(ctx, err)
		}
	}
	return nil
}

func (r *PolicyVersion) CompareAndSwapHead(ctx context.Context, next legal.TypeHead) (bool, error) {
	doc := newPolicyTypeDoc(next)
	if next.Revision == 1 {
		if _, err := r.types.InsertOne(ctx, doc); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return false, nil
			}
			return false, rerror.ErrInternalByWithContext(ctx, err)
		}
		return true, nil
	}
	res, err := r.types.ReplaceOne(ctx, bson.M{"_id": doc.Type, "revision": next.Revision - 1}, doc)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return res.MatchedCount == 1, nil
}

func (r *PolicyVersion) FindHead(ctx context.Context, t legal.Type) (legal.TypeHead, error) {
	var doc policyTypeDoc
	err := r.types.FindOne(ctx, bson.M{"_id": string(t)}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return legal.TypeHead{Type: t}, nil
	}
	if err != nil {
		return legal.TypeHead{}, rerror.ErrInternalByWithContext(ctx, err)
	}
	h, err := doc.model()
	if err != nil {
		return legal.TypeHead{}, rerror.ErrInternalByWithContext(ctx, err)
	}
	return h, nil
}

func (r *PolicyVersion) FindHeads(ctx context.Context) ([]legal.TypeHead, error) {
	var docs []policyTypeDoc
	if err := findAll(ctx, r.types, &docs); err != nil {
		return nil, err
	}
	res := make([]legal.TypeHead, 0, len(docs))
	for _, d := range docs {
		h, err := d.model()
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, h)
	}
	return res, nil
}

func (r *PolicyVersion) FindVersions(ctx context.Context) ([]legal.VersionRecord, error) {
	var docs []policyVersionDoc
	if err := findAll(ctx, r.versions, &docs); err != nil {
		return nil, err
	}
	res := make([]legal.VersionRecord, 0, len(docs))
	for _, d := range docs {
		rec, err := versionDoc{
			EffectiveAt: d.EffectiveAt,
			Required:    d.Required,
			Title:       d.Title,
			URL:         d.URL,
			Version:     d.Version,
		}.model(legal.Type(d.Type))
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, rec)
	}
	return res, nil
}

func findAll[T any](ctx context.Context, c *mongo.Collection, out *[]T) error {
	cur, err := c.Find(ctx, bson.M{})
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	if err := cur.All(ctx, out); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func newPolicyTypeDoc(h legal.TypeHead) policyTypeDoc {
	doc := policyTypeDoc{
		Applied:   appliedDoc{Digest: h.Applied.Digest, Version: h.Applied.Version.String()},
		AppliedAt: h.AppliedAt,
		Revision:  h.Revision,
		Type:      string(h.Type),
		Versions:  []versionDoc{},
	}
	for _, v := range h.SortedVersions() {
		doc.Versions = append(doc.Versions, versionDoc{
			EffectiveAt: v.EffectiveAt,
			Required:    v.Required,
			Title:       v.Title,
			URL:         v.URL,
			Version:     v.Version.String(),
			WithdrawnAt: v.WithdrawnAt,
		})
	}
	return doc
}

func (d policyTypeDoc) model() (legal.TypeHead, error) {
	t := legal.Type(d.Type)
	v, err := legal.ParseVersion(d.Applied.Version)
	if err != nil {
		return legal.TypeHead{}, err
	}
	h := legal.TypeHead{
		Applied:   legal.Applied{Digest: d.Applied.Digest, Version: v},
		AppliedAt: d.AppliedAt,
		Revision:  d.Revision,
		Type:      t,
		Versions:  make(map[legal.Version]legal.HeadVersion, len(d.Versions)),
	}
	for _, vd := range d.Versions {
		rec, err := vd.model(t)
		if err != nil {
			return legal.TypeHead{}, err
		}
		h.Versions[rec.Version] = legal.HeadVersion{VersionRecord: rec, WithdrawnAt: vd.WithdrawnAt}
	}
	return h, nil
}

func (d versionDoc) model(t legal.Type) (legal.VersionRecord, error) {
	v, err := legal.ParseVersion(d.Version)
	if err != nil {
		return legal.VersionRecord{}, err
	}
	return legal.VersionRecord{
		Ref:         legal.Ref{Type: t, Version: v},
		EffectiveAt: d.EffectiveAt,
		Required:    d.Required,
		Title:       d.Title,
		URL:         d.URL,
	}, nil
}
