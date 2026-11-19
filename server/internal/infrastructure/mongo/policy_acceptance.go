package mongo

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	duplicateKeyCode            = 11000
	policyAcceptancesCollection = "policy_acceptances"
)

type PolicyAcceptance struct {
	c *mongo.Collection
}

func NewPolicyAcceptance(db *mongo.Database) *PolicyAcceptance {
	return &PolicyAcceptance{c: db.Collection(policyAcceptancesCollection)}
}

type policyAcceptanceDoc struct {
	AcceptedAt time.Time `bson:"acceptedAt"`
	IP         string    `bson:"ip"`
	Origin     string    `bson:"origin"`
	Type       string    `bson:"type"`
	User       string    `bson:"user"`
	UserAgent  string    `bson:"userAgent"`
	Version    string    `bson:"version"`
}

func (r *PolicyAcceptance) FindByUser(ctx context.Context, u id.UserID) ([]legal.Acceptance, error) {
	cur, err := r.c.Find(ctx, bson.M{"user": u.String()})
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	var docs []policyAcceptanceDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.Acceptance, 0, len(docs))
	for _, d := range docs {
		a, err := d.model()
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, a)
	}
	return res, nil
}

func (r *PolicyAcceptance) InsertIfAbsent(ctx context.Context, as []legal.Acceptance) error {
	if len(as) == 0 {
		return nil
	}
	docs := make([]any, len(as))
	for i, a := range as {
		docs[i] = newPolicyAcceptanceDoc(a)
	}
	_, err := r.c.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	if err != nil && !onlyDuplicateKeys(err) {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *PolicyAcceptance) RemoveByUser(ctx context.Context, u id.UserID) error {
	if _, err := r.c.DeleteMany(ctx, bson.M{"user": u.String()}); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func onlyDuplicateKeys(err error) bool {
	var bwe mongo.BulkWriteException
	if !errors.As(err, &bwe) || bwe.WriteConcernError != nil || len(bwe.WriteErrors) == 0 {
		return false
	}
	for _, we := range bwe.WriteErrors {
		if we.Code != duplicateKeyCode {
			return false
		}
	}
	return true
}

func newPolicyAcceptanceDoc(a legal.Acceptance) policyAcceptanceDoc {
	d := policyAcceptanceDoc{
		AcceptedAt: a.AcceptedAt,
		Origin:     a.Client.Origin,
		Type:       string(a.Type),
		User:       a.UserID.String(),
		UserAgent:  a.Client.UserAgent,
		Version:    a.Version.String(),
	}
	if a.Client.IP.IsValid() {
		d.IP = a.Client.IP.String()
	}
	return d
}

func (d policyAcceptanceDoc) model() (legal.Acceptance, error) {
	u, err := id.UserIDFrom(d.User)
	if err != nil {
		return legal.Acceptance{}, err
	}
	v, err := legal.ParseVersion(d.Version)
	if err != nil {
		return legal.Acceptance{}, err
	}
	a := legal.Acceptance{
		AcceptedAt: d.AcceptedAt,
		Client:     legal.Client{Origin: d.Origin, UserAgent: d.UserAgent},
		Ref:        legal.Ref{Type: legal.Type(d.Type), Version: v},
		UserID:     u,
	}
	if d.IP != "" {
		if a.Client.IP, err = netip.ParseAddr(d.IP); err != nil {
			return legal.Acceptance{}, err
		}
	}
	return a, nil
}
