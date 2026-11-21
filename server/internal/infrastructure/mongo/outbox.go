package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mailOutboxCollection = "mail_outbox"

type Outbox struct {
	c *mongo.Collection
}

func NewOutbox(db *mongo.Database) *Outbox {
	return &Outbox{c: db.Collection(mailOutboxCollection)}
}

type outboxClaimDoc struct {
	Token string    `bson:"token"`
	Until time.Time `bson:"until"`
}

type outboxDoc struct {
	Address       string            `bson:"address,omitempty"`
	Attempts      int               `bson:"attempts"`
	Claim         *outboxClaimDoc   `bson:"claim,omitempty"`
	CreatedAt     time.Time         `bson:"createdAt"`
	EndedAt       *time.Time        `bson:"endedAt,omitempty"`
	Key           string            `bson:"_id"`
	Kind          string            `bson:"kind"`
	NextAttemptAt time.Time         `bson:"nextAttemptAt"`
	Reason        string            `bson:"reason,omitempty"`
	State         string            `bson:"state"`
	Subject       map[string]string `bson:"subject,omitempty"`
	UpdatedAt     time.Time         `bson:"updatedAt"`
	User          string            `bson:"user,omitempty"`
}

func (r *Outbox) ClaimDue(ctx context.Context, now time.Time, c outbox.Claim) (*outbox.Record, error) {
	var d outboxDoc
	err := r.c.FindOneAndUpdate(ctx,
		bson.M{"state": string(outbox.StatePending), "nextAttemptAt": bson.M{"$lte": now}},
		bson.M{
			"$set": bson.M{"state": string(outbox.StateSending), "claim": outboxClaimDoc(c), "updatedAt": now},
			"$inc": bson.M{"attempts": 1},
		},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "nextAttemptAt", Value: 1}}).SetReturnDocument(options.After),
	).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	rec, err := d.model()
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return rec, nil
}

func (r *Outbox) Complete(ctx context.Context, k outbox.Key, token string, t outbox.Transition, now time.Time) (bool, error) {
	set := bson.M{"state": string(t.To), "updatedAt": now}
	if t.Reason != "" {
		set["reason"] = string(t.Reason)
	}
	if t.To == outbox.StatePending {
		set["nextAttemptAt"] = t.NextAttemptAt
	}
	if t.To.Ended() {
		set["endedAt"] = now
	}
	res, err := r.c.UpdateOne(ctx,
		bson.M{"_id": string(k), "state": string(outbox.StateSending), "claim.token": token, "claim.until": bson.M{"$gt": now}},
		bson.M{"$set": set, "$unset": bson.M{"claim": ""}},
	)
	if err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	return res.MatchedCount == 1, nil
}

func (r *Outbox) CountByNotice(ctx context.Context, n id.PolicyNoticeID) (map[outbox.State]int, error) {
	cur, err := r.c.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"subject." + outbox.SubjectNotice: n.String()}}},
		{{Key: "$group", Value: bson.M{"_id": "$state", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	var rows []struct {
		N     int    `bson:"n"`
		State string `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	counts := outbox.EmptyCounts()
	for _, row := range rows {
		counts[outbox.State(row.State)] = row.N
	}
	return counts, nil
}

func (r *Outbox) RemoveFailedByNotice(ctx context.Context, n id.PolicyNoticeID, reason outbox.Reason) error {
	_, err := r.c.DeleteMany(ctx, bson.M{"subject." + outbox.SubjectNotice: n.String(), "state": string(outbox.StateFailed), "reason": string(reason)})
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Outbox) Enqueue(ctx context.Context, rs []outbox.Record) error {
	if len(rs) == 0 {
		return nil
	}
	docs := make([]any, len(rs))
	for i, rec := range rs {
		docs[i] = newOutboxDoc(rec)
	}
	_, err := r.c.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	if err != nil && !onlyDuplicateKeys(err) {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Outbox) FindByKey(ctx context.Context, k outbox.Key) (*outbox.Record, error) {
	var d outboxDoc
	err := r.c.FindOne(ctx, bson.M{"_id": string(k)}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, rerror.ErrNotFound
	}
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	rec, err := d.model()
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return rec, nil
}

func (r *Outbox) RemoveByUser(ctx context.Context, u id.UserID) error {
	if _, err := r.c.DeleteMany(ctx, bson.M{"user": u.String()}); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Outbox) Sweep(ctx context.Context, now time.Time, ttl time.Duration) (outbox.SweepResult, error) {
	var res outbox.SweepResult
	expired, err := r.c.UpdateMany(ctx,
		bson.M{"state": string(outbox.StateSending), "claim.until": bson.M{"$lte": now}},
		bson.M{
			"$set":   bson.M{"state": string(outbox.StateUnknown), "reason": string(outbox.ReasonClaimExpired), "endedAt": now, "updatedAt": now},
			"$unset": bson.M{"claim": ""},
		})
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.Expired = int(expired.ModifiedCount)
	cutoff := now.Add(-ttl)
	timedOut, err := r.c.UpdateMany(ctx,
		bson.M{"state": string(outbox.StatePending), "createdAt": bson.M{"$lte": cutoff}},
		bson.M{"$set": bson.M{"state": string(outbox.StateFailed), "reason": string(outbox.ReasonTTL), "endedAt": now, "updatedAt": now}})
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.TimedOut = int(timedOut.ModifiedCount)
	deleted, err := r.c.DeleteMany(ctx, bson.M{"endedAt": bson.M{"$lte": cutoff}})
	if err != nil {
		return res, rerror.ErrInternalByWithContext(ctx, err)
	}
	res.Deleted = int(deleted.DeletedCount)
	return res, nil
}

func newOutboxDoc(rec outbox.Record) outboxDoc {
	d := outboxDoc{
		Attempts:      rec.Attempts,
		CreatedAt:     rec.CreatedAt,
		EndedAt:       rec.EndedAt,
		Key:           string(rec.Key),
		Kind:          string(rec.Kind),
		NextAttemptAt: rec.NextAttemptAt,
		Reason:        string(rec.Reason),
		State:         string(rec.State),
		Subject:       rec.Subject,
		UpdatedAt:     rec.UpdatedAt,
	}
	if rec.Claim != nil {
		d.Claim = &outboxClaimDoc{Token: rec.Claim.Token, Until: rec.Claim.Until}
	}
	if u, ok := rec.Recipient.User(); ok {
		d.User = u.String()
	} else if a, ok := rec.Recipient.Address(); ok {
		d.Address = a
	}
	return d
}

func (d outboxDoc) model() (*outbox.Record, error) {
	rec := &outbox.Record{
		Attempts:      d.Attempts,
		CreatedAt:     d.CreatedAt.UTC(),
		Key:           outbox.Key(d.Key),
		Kind:          mail.Kind(d.Kind),
		NextAttemptAt: d.NextAttemptAt.UTC(),
		Reason:        outbox.Reason(d.Reason),
		State:         outbox.State(d.State),
		Subject:       d.Subject,
		UpdatedAt:     d.UpdatedAt.UTC(),
	}
	if d.Claim != nil {
		rec.Claim = &outbox.Claim{Token: d.Claim.Token, Until: d.Claim.Until.UTC()}
	}
	if d.EndedAt != nil {
		e := d.EndedAt.UTC()
		rec.EndedAt = &e
	}
	if d.User != "" {
		u, err := id.UserIDFrom(d.User)
		if err != nil {
			return nil, err
		}
		rec.Recipient = outbox.ToUser(u)
	} else {
		rec.Recipient = outbox.ToAddress(d.Address)
	}
	return rec, nil
}
