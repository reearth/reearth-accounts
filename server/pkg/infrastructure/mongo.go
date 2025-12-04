package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	internalRepo "github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearthx/mongox"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

// NewMongoUser creates a new MongoDB-backed User repository
func NewMongoUser(client *mongox.Client) repo.User {
	internal := mongo.NewUser(client)
	return NewUserAdapter(internal)
}

// NewMongoWorkspace creates a new MongoDB-backed Workspace repository
func NewMongoWorkspace(client *mongox.Client) repo.Workspace {
	internal := mongo.NewWorkspace(client)
	return NewWorkspaceAdapter(internal)
}

// NewMongoUserWithHost creates a new MongoDB-backed User repository with host
func NewMongoUserWithHost(client *mongox.Client, host string) repo.User {
	internal := mongo.NewUserWithHost(client, host)
	return NewUserAdapter(internal)
}

// New creates a new MongoDB-backed repository container
// This matches the signature used by reearthx accountmongo
func New(ctx context.Context, client *mongodriver.Client, databaseName string, useTransaction, needCompat bool, users []repo.User) (*repo.Container, error) {
	// Get database from client
	db := client.Database(databaseName)

	// Convert pkg users to internal users (reverse adapter)
	internalUsers := make([]internalRepo.User, len(users))
	for i, u := range users {
		if ua, ok := u.(*userAdapter); ok {
			// If it's already a userAdapter, extract the internal implementation
			internalUsers[i] = ua.internal
		} else {
			// This shouldn't happen in normal usage, but handle it gracefully
			// Create a temporary adapter and extract internal
			internal := mongo.NewUser(mongox.NewClient(databaseName, client))
			internalUsers[i] = internal
		}
	}

	// Call internal New
	internalContainer, err := mongo.New(ctx, db, useTransaction, needCompat, internalUsers)
	if err != nil {
		return nil, err
	}

	// Convert internal container to pkg container
	pkgUsers := make([]repo.User, len(internalContainer.Users))
	for i, u := range internalContainer.Users {
		pkgUsers[i] = NewUserAdapter(u)
	}

	return &repo.Container{
		User:        NewUserAdapter(internalContainer.User),
		Workspace:   NewWorkspaceAdapter(internalContainer.Workspace),
		Role:        internalContainer.Role,        // Same interface
		Permittable: internalContainer.Permittable, // Same interface
		Transaction: internalContainer.Transaction,
		Users:       pkgUsers,
	}, nil
}
