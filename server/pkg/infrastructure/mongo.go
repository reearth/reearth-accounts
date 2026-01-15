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

// NewMongoWorkspace creates a new MongoDB-backed Workspace repository
func NewMongoWorkspace(client *mongox.Client) workspace.Repo {
	return mongo.NewWorkspace(client)
}

// NewMongoUserWithHost creates a new MongoDB-backed User repository with host
func NewMongoUserWithHost(client *mongox.Client, host string) user.Repo {
	return mongo.NewUserWithHost(client, host)
}

// New creates a new MongoDB-backed repository container
// This matches the signature used by reearthx accountmongo
func New(ctx context.Context, client *mongodriver.Client, databaseName string, useTransaction, needCompat bool, users []user.Repo) (*Container, error) {
	// Get database from client
	db := client.Database(databaseName)

	// Call internal New
	internalContainer, err := mongo.New(ctx, db, useTransaction, needCompat, users)
	if err != nil {
		return nil, err
	}

	return &Container{
		User:        internalContainer.User,
		Workspace:   internalContainer.Workspace,
		Role:        internalContainer.Role,
		Permittable: internalContainer.Permittable,
		Transaction: internalContainer.Transaction,
		Users:       internalContainer.Users,
	}, nil
}
