package gqlerror

import (
	"errors"
	"testing"

	"github.com/hasura/go-graphql-client"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFound(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsNotFound(nil))
	})

	t.Run("sentinel ErrNotFound", func(t *testing.T) {
		assert.True(t, IsNotFound(ErrNotFound))
	})

	t.Run("graphql not found error", func(t *testing.T) {
		err := graphql.Errors{{Message: "input: findUserByAlias not found"}}
		assert.True(t, IsNotFound(err))
	})

	t.Run("graphql not found error is case-insensitive", func(t *testing.T) {
		err := graphql.Errors{{Message: "Record Not Found"}}
		assert.True(t, IsNotFound(err))
	})

	t.Run("non-graphql error containing not found is not matched", func(t *testing.T) {
		err := errors.New("HTTP 404 Not Found")
		assert.False(t, IsNotFound(err))
	})

	t.Run("unrelated graphql error", func(t *testing.T) {
		err := graphql.Errors{{Message: "internal server error"}}
		assert.False(t, IsNotFound(err))
	})
}

func TestIsUnauthorized(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsUnauthorized(nil))
	})

	t.Run("unauthorized error", func(t *testing.T) {
		assert.True(t, IsUnauthorized(ErrUnauthorized))
	})
}
