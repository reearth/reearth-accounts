package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

// NewMongoUser creates a new MongoDB-backed User repository
func NewMongoUser(client *mongox.Client) user.Repo {
	return mongo.NewUser(client)
}

// NewMongoUserWithHost creates a new MongoDB-backed User repository with host
func NewMongoUserWithHost(client *mongox.Client, host string) user.Repo {
	return mongo.NewUserWithHost(client, host)
}

// NewMongoWorkspace creates a new MongoDB-backed Workspace repository
func NewMongoWorkspace(client *mongox.Client) workspace.Repo {
	return mongo.NewWorkspace(client)
}

// New creates a new MongoDB-backed repository container
func New(ctx context.Context, client *mongodriver.Client, databaseName string, useTransaction, needCompat bool) (*Container, error) {
	db := client.Database(databaseName)

	internalContainer, err := mongo.New(ctx, db, useTransaction, needCompat)
	if err != nil {
		return nil, err
	}

	return &Container{
		Config:      internalContainer.Config,
		Permittable: internalContainer.Permittable,
		Role:        internalContainer.Role,
		Transaction: internalContainer.Transaction,
		User:        internalContainer.User,
		Workspace:   internalContainer.Workspace,
	}, nil
}
