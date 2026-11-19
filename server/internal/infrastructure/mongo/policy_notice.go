package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const policyNoticesCollection = "policy_notices"

type PolicyNotice struct {
	c *mongo.Collection
}

func NewPolicyNotice(db *mongo.Database) *PolicyNotice {
	return &PolicyNotice{c: db.Collection(policyNoticesCollection)}
}

type noticeLeaseDoc struct {
	Token string    `bson:"token"`
	Until time.Time `bson:"until"`
}

type noticeFanoutDoc struct {
	Cursor string          `bson:"cursor"`
	Done   bool            `bson:"done"`
	Lease  *noticeLeaseDoc `bson:"lease,omitempty"`
}

type policyNoticeDoc struct {
	Audience    string          `bson:"audience"`
	Bump        string          `bson:"bump"`
	CreatedAt   time.Time       `bson:"createdAt"`
	Fanout      noticeFanoutDoc `bson:"fanout"`
	ID          string          `bson:"_id"`
	RequestedBy string          `bson:"requestedBy"`
	Type        string          `bson:"type"`
	Version     string          `bson:"version"`
}

func (r *PolicyNotice) AdvanceFanout(ctx context.Context, n id.PolicyNoticeID, lease legal.Lease, cursor string, done bool) (bool, error) {
	update := bson.M{"$set": bson.M{"fanout.cursor": cursor, "fanout.done": done, "fanout.lease": noticeLeaseDoc(lease)}}
	if done {
		update = bson.M{"$set": bson.M{"fanout.cursor": cursor, "fanout.done": true}, "$unset": bson.M{"fanout.lease": ""}}
	}
	res, err := r.c.UpdateOne(ctx, bson.M{"_id": n.String(), "fanout.lease.token": lease.Token}, update)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return res.MatchedCount == 1, nil
}

func (r *PolicyNotice) ClaimFanout(ctx context.Context, now time.Time, lease legal.Lease) (*legal.Notice, error) {
	var d policyNoticeDoc
	err := r.c.FindOneAndUpdate(ctx,
		bson.M{"fanout.done": false, "$or": bson.A{
			bson.M{"fanout.lease": bson.M{"$exists": false}},
			bson.M{"fanout.lease.until": bson.M{"$lte": now}},
		}},
		bson.M{"$set": bson.M{"fanout.lease": noticeLeaseDoc(lease)}},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetReturnDocument(options.After),
	).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	n, err := d.model()
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return &n, nil
}

func (r *PolicyNotice) Create(ctx context.Context, n legal.Notice) (legal.Notice, bool, error) {
	d := newPolicyNoticeDoc(n)
	_, err := r.c.InsertOne(ctx, d)
	if err == nil {
		return n, true, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return legal.Notice{}, false, rerror.ErrInternalByWithContext(ctx, err)
	}
	var stored policyNoticeDoc
	if err := r.c.FindOne(ctx, bson.M{"type": d.Type, "version": d.Version, "audience": d.Audience}).Decode(&stored); err != nil {
		return legal.Notice{}, false, rerror.ErrInternalByWithContext(ctx, err)
	}
	m, err := stored.model()
	if err != nil {
		return legal.Notice{}, false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return m, false, nil
}

func (r *PolicyNotice) FindByRef(ctx context.Context, ref legal.Ref) ([]legal.Notice, error) {
	cur, err := r.c.Find(ctx, bson.M{"type": string(ref.Type), "version": ref.Version.String()},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	var docs []policyNoticeDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.Notice, 0, len(docs))
	for _, d := range docs {
		n, err := d.model()
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, n)
	}
	return res, nil
}

func (r *PolicyNotice) ReleaseFanout(ctx context.Context, n id.PolicyNoticeID, token string) error {
	_, err := r.c.UpdateOne(ctx, bson.M{"_id": n.String(), "fanout.lease.token": token}, bson.M{"$unset": bson.M{"fanout.lease": ""}})
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func newPolicyNoticeDoc(n legal.Notice) policyNoticeDoc {
	d := policyNoticeDoc{
		Audience:    string(n.Audience),
		Bump:        string(n.Bump),
		CreatedAt:   n.CreatedAt,
		Fanout:      noticeFanoutDoc{Cursor: n.Fanout.Cursor, Done: n.Fanout.Done},
		ID:          n.ID.String(),
		RequestedBy: n.RequestedBy.String(),
		Type:        string(n.Ref.Type),
		Version:     n.Ref.Version.String(),
	}
	if l := n.Fanout.Lease; l != nil {
		d.Fanout.Lease = &noticeLeaseDoc{Token: l.Token, Until: l.Until}
	}
	return d
}

func (d policyNoticeDoc) model() (legal.Notice, error) {
	nid, err := id.PolicyNoticeIDFrom(d.ID)
	if err != nil {
		return legal.Notice{}, err
	}
	by, err := id.AdminUserIDFrom(d.RequestedBy)
	if err != nil {
		return legal.Notice{}, err
	}
	t, err := legal.ParseType(d.Type)
	if err != nil {
		return legal.Notice{}, err
	}
	v, err := legal.ParseVersion(d.Version)
	if err != nil {
		return legal.Notice{}, err
	}
	n := legal.Notice{
		Audience:    legal.Audience(d.Audience),
		Bump:        legal.Bump(d.Bump),
		CreatedAt:   d.CreatedAt.UTC(),
		Fanout:      legal.Fanout{Cursor: d.Fanout.Cursor, Done: d.Fanout.Done},
		ID:          nid,
		Ref:         legal.Ref{Type: t, Version: v},
		RequestedBy: by,
	}
	if l := d.Fanout.Lease; l != nil {
		n.Fanout.Lease = &legal.Lease{Token: l.Token, Until: l.Until.UTC()}
	}
	return n, nil
}
