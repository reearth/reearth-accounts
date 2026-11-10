//go:generate go run github.com/99designs/gqlgen

package gql

import (
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
)

// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

var ErrNotImplemented = errors.New("not impleneted yet")
var ErrUnauthorized = errors.New("unauthorized")

type Resolver struct {
	LoginTime adapter.LoginTimeSource
	Photos    adapter.PhotoSigner
}

func NewResolver(photos adapter.PhotoSigner, loginTime adapter.LoginTimeSource) ResolverRoot {
	return &Resolver{
		LoginTime: loginTime,
		Photos:    photos,
	}
}
