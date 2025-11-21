package infrastructure

import (
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearthx/mongox"
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
