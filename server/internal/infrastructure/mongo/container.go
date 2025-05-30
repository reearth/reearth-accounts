package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/util"
	"go.mongodb.org/mongo-driver/mongo"
)

func New(ctx context.Context, db *mongo.Database, useTransaction, needCompat bool, users []repo.User) (*repo.Container, error) {
	client := mongox.NewClientWithDatabase(db)
	if useTransaction {
		client = client.WithTransaction()
	}

	var ws repo.Workspace
	if needCompat {
		ws = NewWorkspaceCompat(client)
	} else {
		ws = NewWorkspace(client)
	}

	lock, err := NewLock(db.Collection("locks"))
	if err != nil {
		return nil, err
	}

	c := &repo.Container{
		User:        NewUser(client),
		Workspace:   ws,
		Role:        NewRole(client),
		Permittable: NewPermittable(client),
		Transaction: client.Transaction(),
		Users:       users,
		Config:      NewConfig(db.Collection("config"), lock),
	}

	// init
	if err := Init(c); err != nil {
		return nil, err
	}

	return c, nil
}

func Init(r *repo.Container) error {
	if r == nil {
		return nil
	}

	ctx := context.Background()
	return util.Try(
		func() error { return r.Role.(*Role).Init(ctx) },
		func() error { return r.Permittable.(*Permittable).Init(ctx) },
	)
}

func createIndexes(ctx context.Context, c *mongox.ClientCollection, keys, uniqueKeys []string) error {
	created, deleted, err := c.Indexes(ctx, keys, uniqueKeys)
	if len(created) > 0 || len(deleted) > 0 {
		log.Infofc(ctx, "mongo: %s: index deleted: %v, created: %v\n", c.Client().Name(), deleted, created)
	}
	return err
}
