package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-key"))
	require.NoError(t, err)
	return s
}

func TestAuthTime(t *testing.T) {
	withToken := func(token string) context.Context {
		return context.WithValue(context.Background(), AuthInfoKey, appx.AuthInfo{Token: token})
	}

	t.Run("claim present", func(t *testing.T) {
		ctx := withToken(signedToken(t, jwt.MapClaims{"sub": "u", "auth_time": 1700000000}))
		got := AuthTime(ctx)
		require.NotNil(t, got)
		assert.True(t, got.Equal(time.Unix(1700000000, 0)))
	})

	t.Run("claim absent", func(t *testing.T) {
		assert.Nil(t, AuthTime(withToken(signedToken(t, jwt.MapClaims{"sub": "u"}))))
	})

	t.Run("claim not a number", func(t *testing.T) {
		assert.Nil(t, AuthTime(withToken(signedToken(t, jwt.MapClaims{"auth_time": "yesterday"}))))
	})

	t.Run("malformed token", func(t *testing.T) {
		assert.Nil(t, AuthTime(withToken("not-a-jwt")))
	})

	t.Run("no auth info", func(t *testing.T) {
		assert.Nil(t, AuthTime(context.Background()))
	})
}
