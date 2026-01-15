package mongo

import (
	"context"
	"fmt"
	"regexp"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	client *mongox.Collection
	host   string
}

func NewUser(client *mongox.Client) user.Repo {
	return &User{client: client.WithCollection("user")}
}

func NewUserWithHost(client *mongox.Client, host string) user.Repo {
	return &User{client: client.WithCollection("user"), host: host}
}

func (r *User) FindAll(ctx context.Context) (user.List, error) {
	res, err := r.find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *User) FindByID(ctx context.Context, id2 user.ID) (*user.User, error) {
	return r.findOne(ctx, bson.M{"id": id2.String()})
}

func (r *User) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	res, err := r.find(ctx, bson.M{
		"id": bson.M{"$in": ids.Strings()},
	})
	if err != nil {
		return nil, err
	}
	return filterUsers(ids, res), nil
}

func (r *User) FindBySub(ctx context.Context, auth0sub string) (*user.User, error) {
	return r.findOne(ctx, bson.M{
		"$or": []bson.M{
			{
				"subs": bson.M{
					"$elemMatch": bson.M{
						"$eq": auth0sub,
					},
				},
			},
			{"auth0sub": auth0sub},
			{
				"auth0sublist": bson.M{ //compat
					"$elemMatch": bson.M{
						"$eq": auth0sub,
					},
				},
			},
		},
	})
}

func (r *User) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return r.findOne(ctx, bson.M{"email": email})
}

func (r *User) FindByName(ctx context.Context, name string) (*user.User, error) {
	return r.findOne(ctx, bson.M{"name": name})
}

func (r *User) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return r.findOne(ctx, bson.M{
		"$or": []bson.M{
			{"email": nameOrEmail},
			{"name": nameOrEmail},
		},
	})
}

func (r *User) FindByAlias(ctx context.Context, alias string) (*user.User, error) {
	return r.findOne(ctx, bson.M{"alias": alias})
}

func (r *User) SearchByKeyword(ctx context.Context, keyword string) (user.List, error) {
	if len(keyword) < 3 {
		return nil, nil
	}
	regex := bson.M{"$regex": primitive.Regex{Pattern: regexp.QuoteMeta(keyword), Options: "i"}}
	return r.find(ctx,
		bson.M{"$or": []bson.M{{"email": regex}, {"name": regex}}},
		options.Find().SetLimit(10).SetSort(bson.M{"name": 1}),
	)
}

func (r *User) FindByVerification(ctx context.Context, code string) (*user.User, error) {
	return r.findOne(ctx, bson.M{
		"verification.code": code,
	})
}

func (r *User) FindByPasswordResetRequest(ctx context.Context, pwdResetToken string) (*user.User, error) {
	return r.findOne(ctx, bson.M{
		"passwordreset.token": pwdResetToken,
	})
}

func (r *User) FindBySubOrCreate(ctx context.Context, u *user.User, sub string) (*user.User, error) {
	userDoc, _ := mongodoc.NewUser(u)
	if err := r.client.Client().FindOneAndUpdate(
		ctx,
		bson.M{
			"$or": []bson.M{
				{
					"subs": bson.M{
						"$elemMatch": bson.M{
							"$eq": sub,
						},
					},
				},
			},
		},
		bson.M{"$setOnInsert": userDoc},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&userDoc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, user.ErrDuplicatedUser
		}
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return userDoc.Model()
}

func (r *User) Create(ctx context.Context, u *user.User) error {
	doc, _ := mongodoc.NewUser(u)
	if _, err := r.client.Client().InsertOne(
		ctx,
		doc,
	); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return user.ErrDuplicatedUser
		}
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *User) Save(ctx context.Context, u *user.User) error {
	if u.Host() != "" {
		return fmt.Errorf("cannot save an user on the different tenant(host=%s)", u.Host())
	}
	doc, id := mongodoc.NewUser(u)
	return r.client.SaveOne(ctx, id, doc)
}

func (r *User) Remove(ctx context.Context, id user.ID) error {
	return r.client.RemoveOne(ctx, bson.M{"id": id.String()})
}

func (r *User) find(ctx context.Context, filter any, options ...*options.FindOptions) (user.List, error) {
	c := mongodoc.NewUserConsumer(r.host)
	if err := r.client.Find(ctx, filter, c, options...); err != nil {
		return nil, err
	}
	return c.Result, nil
}

func (r *User) findOne(ctx context.Context, filter any) (*user.User, error) {
	c := mongodoc.NewUserConsumer(r.host)
	if err := r.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result[0], nil
}

func filterUsers(ids []user.ID, rows []*user.User) []*user.User {
	res := make([]*user.User, 0, len(ids))
	for _, id := range ids {
		var r2 *user.User
		for _, r := range rows {
			if r.ID() == id {
				r2 = r
				break
			}
		}
		res = append(res, r2)
	}
	return res
}
