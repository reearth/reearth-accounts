package auth

import (
	"context"

	"github.com/reearth/reearthx/appx"
)

type authInfoKey struct{}

// AuthInfoKey is the shared context key used by the auth middleware to store
// the authenticated appx.AuthInfo, and read by lower layers (e.g. the cerbos
// adapter). It lives here so neither infrastructure nor any bounded-context
// adapter needs to depend on another context's presentation layer.
var AuthInfoKey = authInfoKey{}

func AttachAuthInfo(ctx context.Context, ai appx.AuthInfo) context.Context {
	return context.WithValue(ctx, AuthInfoKey, ai)
}

func GetAuthInfo(ctx context.Context) *appx.AuthInfo {
	if v := ctx.Value(AuthInfoKey); v != nil {
		if v2, ok := v.(appx.AuthInfo); ok {
			return &v2
		}
	}
	return nil
}
