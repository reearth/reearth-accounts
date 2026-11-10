package adapter

import (
	"context"
	"math"
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

func TestLoginTimeSource_Read(t *testing.T) {
	withToken := func(token string) context.Context {
		return context.WithValue(context.Background(), AuthInfoKey, appx.AuthInfo{Token: token})
	}

	t.Run("claim present", func(t *testing.T) {
		ctx := withToken(signedToken(t, jwt.MapClaims{"sub": "u", "auth_time": 1700000000}))
		got := LoginTimeSource{}.Read(ctx)
		require.NotNil(t, got)
		assert.True(t, got.Equal(time.Unix(1700000000, 0)))
	})

	for name, prefix := range map[string]string{
		"exact Bearer prefix left in place": "Bearer ",
		"lowercase scheme":                  "bearer ",
		"double space after the scheme":     "Bearer  ",
	} {
		t.Run(name, func(t *testing.T) {
			got := LoginTimeSource{}.Read(withToken(prefix + signedToken(t, jwt.MapClaims{"auth_time": 1700000000})))
			require.NotNil(t, got)
			assert.True(t, got.Equal(time.Unix(1700000000, 0)))
		})
	}

	t.Run("empty token", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{}.Read(withToken("")))
	})

	t.Run("claim absent", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{}.Read(withToken(signedToken(t, jwt.MapClaims{"sub": "u"}))))
	})

	t.Run("claim not a number", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{}.Read(withToken(signedToken(t, jwt.MapClaims{"auth_time": "yesterday"}))))
	})

	t.Run("malformed token", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{}.Read(withToken("not-a-jwt")))
	})

	const claim = "https://example.com/auth_time"

	t.Run("configured claim wins over auth_time", func(t *testing.T) {
		got := LoginTimeSource{Claim: claim}.Read(withToken(signedToken(t, jwt.MapClaims{claim: 1700000000, "auth_time": 1600000000})))
		require.NotNil(t, got)
		assert.True(t, got.Equal(time.Unix(1700000000, 0)))
	})

	t.Run("configured claim absent falls back to auth_time", func(t *testing.T) {
		got := LoginTimeSource{Claim: claim}.Read(withToken(signedToken(t, jwt.MapClaims{"auth_time": 1600000000})))
		require.NotNil(t, got)
		assert.True(t, got.Equal(time.Unix(1600000000, 0)))
	})

	t.Run("neither claim", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{Claim: claim}.Read(withToken(signedToken(t, jwt.MapClaims{"sub": "u"}))))
	})

	t.Run("configured claim not a number does not fall back to auth_time", func(t *testing.T) {
		recent := time.Now().Unix()
		for name, v := range map[string]any{"string": "1700000000", "null": nil, "object": map[string]any{"t": 1}} {
			got := LoginTimeSource{Claim: claim}.Read(withToken(signedToken(t, jwt.MapClaims{claim: v, "auth_time": recent})))
			assert.Nil(t, got, name)
		}
	})

	t.Run("value beyond int64 seconds", func(t *testing.T) {
		for _, v := range []float64{1e19, -1e19, math.MaxInt64} {
			assert.Nil(t, LoginTimeSource{}.Read(withToken(signedToken(t, jwt.MapClaims{"auth_time": v}))), v)
		}
	})

	t.Run("no auth info", func(t *testing.T) {
		assert.Nil(t, LoginTimeSource{}.Read(context.Background()))
	})
}
