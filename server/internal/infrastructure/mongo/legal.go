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

// PolicyVersion uses single-document conditional writes only, so it needs no
// transaction (standalone MongoDB has none).
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
	AppliedAt time.Time       `bson:"appliedAt"`
	Revision  int64           `bson:"revision"`
	Type      string          `bson:"_id"`
	Upcoming  *scheduledDoc   `bson:"upcoming"`
	Withdrawn []withdrawalDoc `bson:"withdrawn"`
}

type scheduledDoc struct {
	EffectiveAt time.Time `bson:"effectiveAt"`
	Version     string    `bson:"version"`
}

type withdrawalDoc struct {
	At      time.Time `bson:"at"`
	Version string    `bson:"version"`
}

type policyVersionDoc struct {
	EffectiveAt time.Time         `bson:"effectiveAt"`
	Required    bool              `bson:"required"`
	Stamp       time.Time         `bson:"stamp"`
	Title       map[string]string `bson:"title"`
	Type        string            `bson:"type"`
	URL         map[string]string `bson:"url"`
	Version     string            `bson:"version"`
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
		v, err := legal.ParseVersion(d.Version)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, legal.VersionRecord{
			Ref:         legal.Ref{Type: legal.Type(d.Type), Version: v},
			EffectiveAt: d.EffectiveAt,
			Required:    d.Required,
			Stamp:       d.Stamp,
			Title:       d.Title,
			URL:         d.URL,
		})
	}
	return res, nil
}

// UpsertVersion matches the stored record only when it may be overwritten.
// When a record exists that does not match, the upsert's insert hits the
// unique (type, version) index, and the duplicate key means "keep it".
func (r *PolicyVersion) UpsertVersion(ctx context.Context, v legal.VersionRecord, now time.Time) error {
	doc := policyVersionDoc{
		EffectiveAt: v.EffectiveAt,
		Required:    v.Required,
		Stamp:       v.Stamp,
		Title:       v.Title,
		Type:        string(v.Type),
		URL:         v.URL,
		Version:     v.Version.String(),
	}
	filter := bson.M{
		"type":        doc.Type,
		"version":     doc.Version,
		"stamp":       bson.M{"$lte": v.Stamp},
		"effectiveAt": bson.M{"$gt": now},
	}
	_, err := r.versions.UpdateOne(ctx, filter, bson.M{"$set": doc}, options.Update().SetUpsert(true))
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
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
		AppliedAt: h.AppliedAt,
		Revision:  h.Revision,
		Type:      string(h.Type),
		Withdrawn: make([]withdrawalDoc, 0, len(h.Withdrawn)),
	}
	if h.Upcoming != nil {
		doc.Upcoming = &scheduledDoc{EffectiveAt: h.Upcoming.EffectiveAt, Version: h.Upcoming.Version.String()}
	}
	for _, w := range h.Withdrawn {
		doc.Withdrawn = append(doc.Withdrawn, withdrawalDoc{At: w.At, Version: w.Version.String()})
	}
	return doc
}

func (d policyTypeDoc) model() (legal.TypeHead, error) {
	h := legal.TypeHead{AppliedAt: d.AppliedAt, Revision: d.Revision, Type: legal.Type(d.Type)}
	if d.Upcoming != nil {
		v, err := legal.ParseVersion(d.Upcoming.Version)
		if err != nil {
			return legal.TypeHead{}, err
		}
		h.Upcoming = &legal.Scheduled{EffectiveAt: d.Upcoming.EffectiveAt, Version: v}
	}
	for _, w := range d.Withdrawn {
		v, err := legal.ParseVersion(w.Version)
		if err != nil {
			return legal.TypeHead{}, err
		}
		h.Withdrawn = append(h.Withdrawn, legal.Withdrawal{At: w.At, Version: v})
	}
	return h, nil
}
