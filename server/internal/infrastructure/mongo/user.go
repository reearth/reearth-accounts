package mongo

import (
	"context"
	"regexp"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	userIndexes       = []string{"subs", "name"}
	userUniqueIndexes = []string{"id", "email"}
)

type User struct {
	client *mongox.ClientCollection
	host   string
}

func NewUser(client *mongox.Client) *User {
	return &User{
		client: client.WithCollection("user"),
	}
}

func NewUserWithHost(client *mongox.Client, host string) repo.User {
	return &User{client: client.WithCollection("user"), host: host}
}

func (u *User) Init() error {
	return createIndexes(context.Background(), u.client, userIndexes, userUniqueIndexes)
}

func (u *User) FindAll(ctx context.Context) (user.List, error) {
	res, err := u.find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (u *User) FindByID(ctx context.Context, id user.ID) (*user.User, error) {
	return u.findOne(ctx, bson.M{
		"id": id.String(),
	})
}

func (u *User) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	res, err := u.find(ctx, bson.M{
		"id": bson.M{"$in": ids.Strings()},
	})
	if err != nil {
		return nil, err
	}
	return filterUsers(ids, res), nil
}

func (u *User) FindBySub(ctx context.Context, auth0sub string) (*user.User, error) {
	return u.findOne(ctx, bson.M{
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

func (u *User) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return u.findOne(ctx, bson.M{"email": email})
}

func (u *User) FindByName(ctx context.Context, name string) (*user.User, error) {
	return u.findOne(ctx, bson.M{"name": name})
}

func (u *User) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return u.findOne(ctx, bson.M{
		"$or": []bson.M{
			{"email": nameOrEmail},
			{"name": nameOrEmail},
		},
	})
}

func (u *User) SearchByKeyword(ctx context.Context, keyword string) (user.List, error) {
	if len(keyword) < 3 {
		return nil, nil
	}
	regex := bson.M{"$regex": primitive.Regex{Pattern: regexp.QuoteMeta(keyword), Options: "i"}}
	return u.find(ctx,
		bson.M{"$or": []bson.M{{"email": regex}, {"name": regex}}},
		options.Find().SetLimit(10).SetSort(bson.M{"name": 1}),
	)
}

func (u *User) FindByVerification(ctx context.Context, code string) (*user.User, error) {
	return u.findOne(ctx, bson.M{
		"verification.code": code,
	})
}

func (u *User) FindByPasswordResetRequest(ctx context.Context, pwdResetToken string) (*user.User, error) {
	return u.findOne(ctx, bson.M{
		"passwordreset.token": pwdResetToken,
	})
}

func (u *User) FindBySubOrCreate(ctx context.Context, usr *user.User, sub string) (*user.User, error) {
	userDoc, _ := mongodoc.NewUser(usr)
	if err := u.client.Client().FindOneAndUpdate(
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
			return nil, repo.ErrDuplicatedUser
		}
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return userDoc.Model()
}

func (u *User) Create(ctx context.Context, usr *user.User) error {
	doc, _ := mongodoc.NewUser(usr)
	if _, err := u.client.Client().InsertOne(
		ctx,
		doc,
	); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return repo.ErrDuplicatedUser
		}
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (u *User) Save(ctx context.Context, usr *user.User) error {
	doc, uId := mongodoc.NewUser(usr)
	return u.client.SaveOne(ctx, uId, doc)
}

func (u *User) Remove(ctx context.Context, usr user.ID) error {
	return u.client.RemoveOne(ctx, bson.M{"id": usr.String()})
}

func (u *User) find(ctx context.Context, filter any, options ...*options.FindOptions) (user.List, error) {
	c := mongodoc.NewUserConsumer(u.host)
	if err := u.client.Find(ctx, filter, c, options...); err != nil {
		return nil, err
	}
	return c.Result, nil
}

func (u *User) findOne(ctx context.Context, filter any) (*user.User, error) {
	c := mongodoc.NewUserConsumer(u.host)
	if err := u.client.FindOne(ctx, filter, c); err != nil {
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
