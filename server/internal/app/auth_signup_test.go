package app

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBypassed(t *testing.T) {
	t.Run("should detect signup mutations", func(t *testing.T) {
		t.Run("signup mutation with operation name", func(t *testing.T) {
			body := `{"query":"mutation SignupUser($input: SignupInput!) { signup(input: $input) { user { id } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("signup mutation without operation name", func(t *testing.T) {
			body := `{"query":"mutation { signup(input: $input) { user { id } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("signupOIDC mutation", func(t *testing.T) {
			body := `{"query":"mutation { signupOIDC(input: $input) { user { id } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("findById query", func(t *testing.T) {
			body := `{"query":"query { findById(id: \"test\") { id } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("findByAlias query", func(t *testing.T) {
			body := `{"query":"query { findByAlias(alias: \"test\") { id } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("createVerification mutation", func(t *testing.T) {
			body := `{"query":"mutation { createVerification(input: {email: \"test@example.com\"}) { success } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("authConfig query", func(t *testing.T) {
			body := `{"query":"query { authConfig { provider } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("findUsersByIdsWithPagination query", func(t *testing.T) {
			body := `{"query":"query { findUsersByIdsWithPagination(ids: [\"id1\"]) { nodes { id } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("signup mutation with whitespace and newlines", func(t *testing.T) {
			body := `{"query":"mutation {\n  signup(input: $input) {\n    user { id }\n  }\n}"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})
	})

	t.Run("should reject bypass keyword injection via comment", func(t *testing.T) {
		body := `{"query":"# signup(\nmutation { updatePermittable(input: {}) { permittable { id } } }"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.False(t, result)
	})

	t.Run("should reject bypass keyword in operation name", func(t *testing.T) {
		body := `{"query":"mutation signup { updatePermittable(input: {}) { permittable { id } } }"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.False(t, result)
	})

	t.Run("should reject bypass keyword as field alias", func(t *testing.T) {
		body := `{"query":"mutation { signup: updatePermittable(input: {}) { permittable { id } } }"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.False(t, result)
	})

	t.Run("should reject mixed bypassed and protected fields", func(t *testing.T) {
		body := `{"query":"mutation { signup(input: {}) { user { id } } updatePermittable(input: {}) { permittable { id } } }"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.False(t, result)
	})

	t.Run("should allow multiple bypassed fields", func(t *testing.T) {
		body := `{"query":"query { authConfig { clientId } findByID(id: \"abc\") { id } }"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.True(t, result)
	})

	t.Run("should not detect non-signup operations", func(t *testing.T) {
		t.Run("other mutation", func(t *testing.T) {
			body := `{"query":"mutation { updateMe(input: $input) { me { id } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("query operation", func(t *testing.T) {
			body := `{"query":"query { me { id } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("GET request with signup mutation", func(t *testing.T) {
			body := `{"query":"mutation { signup(input: $input) { user { id } } }"}`
			req, err := http.NewRequest(http.MethodGet, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("invalid JSON body", func(t *testing.T) {
			body := `invalid json`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("empty body", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString("")))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})
	})
}
