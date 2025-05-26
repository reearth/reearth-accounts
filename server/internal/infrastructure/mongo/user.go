package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

type User struct {
	client *mongox.ClientCollection
}

func NewUser(client *mongox.Client) *User {
	return &User{
		client: client.WithCollection("user"),
	}
}

func (u *User) FindByID(ctx context.Context, id user.ID) (*user.User, error) {
	return u.findOne(ctx, bson.M{
		"id": id.String(),
	})
}

func (u *User) Save(ctx context.Context, usr *user.User) error {
	doc, uId := mongodoc.NewUser(*usr)
	return u.client.SaveOne(ctx, uId, doc)
}

func (u *User) findOne(ctx context.Context, filter any) (*user.User, error) {
	c := mongodoc.NewUserConsumer()
	if err := u.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result[0], nil
}
