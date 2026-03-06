//go:generate go run github.com/99designs/gqlgen

package gql

import (
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
)

// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

var ErrNotImplemented = errors.New("not impleneted yet")
var ErrUnauthorized = errors.New("unauthorized")

type Resolver struct {
	Authenticator gateway.Authenticator
	Storage       gateway.Storage
}

func NewResolver(storage gateway.Storage, authenticator gateway.Authenticator) ResolverRoot {
	return &Resolver{
		Authenticator: authenticator,
		Storage:       storage,
	}
}
