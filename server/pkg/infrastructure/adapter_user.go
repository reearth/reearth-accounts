package infrastructure

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// NewUserAdapter returns the user.Repo directly since internal and pkg now use the same interface
func NewUserAdapter(internal user.Repo) user.Repo {
	return internal
}
