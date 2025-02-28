package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	newPermittableIndexes       = []string{}
	newPermittableUniqueIndexes = []string{"id", "userid"}
)

type Permittable struct {
	client *mongox.ClientCollection
}

func NewPermittable(client *mongox.Client) *Permittable {
	return &Permittable{
		client: client.WithCollection("permittable"),
	}
}

func (r *Permittable) Init(ctx context.Context) error {
	return createIndexes(ctx, r.client, newPermittableIndexes, newPermittableUniqueIndexes)
}

func (r *Permittable) FindByUserID(ctx context.Context, id user.ID) (*permittable.Permittable, error) {
	return r.findOne(ctx, bson.M{
		"userid": id.String(),
	})
}

func (r *Permittable) FindByUserIDs(ctx context.Context, ids user.IDList) (permittable.List, error) {
	return r.find(ctx, bson.M{
		"userid": bson.M{"$in": ids.Strings()},
	})
}

func (r *Permittable) FindByRoleID(ctx context.Context, roleId id.RoleID) (permittable.List, error) {
	return r.find(ctx, bson.M{
		"roleids": bson.M{"$in": []string{roleId.String()}},
	})
}

func (r *Permittable) Save(ctx context.Context, permittable permittable.Permittable) error {
	doc, gId := mongodoc.NewPermittable(permittable)
	return r.client.SaveOne(ctx, gId, doc)
}

func (r *Permittable) find(ctx context.Context, filter any) (permittable.List, error) {
	c := mongodoc.NewPermittableConsumer()
	if err := r.client.Find(ctx, filter, c); err != nil {
		return nil, err
	}
	if len(c.Result) == 0 {
		return nil, rerror.ErrNotFound
	}
	return (permittable.List)(c.Result), nil
}

func (r *Permittable) findOne(ctx context.Context, filter any) (*permittable.Permittable, error) {
	c := mongodoc.NewPermittableConsumer()
	if err := r.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	if len(c.Result) == 0 {
		return nil, rerror.ErrNotFound
	}
	return c.Result[0], nil
}
