package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/mongo"
)

func New(ctx context.Context, db *mongo.Database, useTransaction, needCompat bool) (*repo.Container, error) {
	client := mongox.NewClientWithDatabase(db)
	if useTransaction {
		client = client.WithTransaction()
	}

	var ws workspace.Repo
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
		Config:      NewConfig(db.Collection("config"), lock),
		Permittable: NewPermittable(client),
		Role:        NewRole(client),
		Transaction: client.Transaction(),
		User:        NewUser(client),
		Workspace:   ws,
	}

	return c, nil
}
