package app

import (
	"bytes"
	"io"
	"net/http"
	"strings"
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
			body := `{"query":"query { authConfig { auth0ClientId authProvider } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("findUsersByIdsWithPagination query", func(t *testing.T) {
			body := `{"query":"query { findUsersByIdsWithPagination(ids: [\"id1\"]) { users { id name alias } totalCount } }"}`
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

	t.Run("should reject sensitive sub-fields on bypassed lookups (SEC-01)", func(t *testing.T) {
		t.Run("findByAlias selecting members is rejected", func(t *testing.T) {
			body := `{"query":"{findByAlias(alias:\"acme\"){id name metadata{billingEmail} members{... on WorkspaceUserMember{userId role user{name email verification{code}}}}}}"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
		})

		t.Run("findByAlias selecting metadata is rejected", func(t *testing.T) {
			body := `{"query":"query { findByAlias(alias: \"acme\") { id name metadata { billingEmail } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
		})

		t.Run("findByAlias selecting only safe fields is allowed", func(t *testing.T) {
			body := `{"query":"query { findByAlias(alias: \"acme\") { id name alias personal } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.True(t, isBypassed(req))
		})

		t.Run("findByID selecting members is rejected", func(t *testing.T) {
			body := `{"query":"query { findByID(id: \"x\") { id members { userId } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
		})

		t.Run("findByIDs selecting members is rejected", func(t *testing.T) {
			body := `{"query":"query { findByIDs(ids: [\"x\"]) { id members { userId } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
		})

		t.Run("findUsersByIDsWithPagination selecting email is rejected", func(t *testing.T) {
			body := `{"query":"query { findUsersByIDsWithPagination(ids: [\"x\"], pagination: {}) { users { id name email verification { code } } totalCount } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
		})

		t.Run("findUsersByIDsWithPagination selecting only safe fields is allowed", func(t *testing.T) {
			body := `{"query":"query { findUsersByIDsWithPagination(ids: [\"x\"], pagination: {}) { users { id name alias } totalCount } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.True(t, isBypassed(req))
		})

		t.Run("chaining findByAlias id harvest into findByIDs member selection is still rejected", func(t *testing.T) {
			body := `{"query":"query { findByAlias(alias: \"acme\") { id } findByIDs(ids: [\"x\"]) { members { userId } } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			assert.False(t, isBypassed(req))
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
		body := `{"query":"query { authConfig { auth0ClientId } findByID(id: \"abc\") { id } }"}`
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

	t.Run("should bypass only the selected operation via operationName", func(t *testing.T) {
		t.Run("operationName selects bypassed operation", func(t *testing.T) {
			body := `{"query":"query Allowed { signup(input: {}) { user { id } } } query Protected { me { id } }", "operationName":"Allowed"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.True(t, result)
		})

		t.Run("operationName selects protected operation", func(t *testing.T) {
			body := `{"query":"query Allowed { signup(input: {}) { user { id } } } query Protected { me { id } }", "operationName":"Protected"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("operationName not found in document", func(t *testing.T) {
			body := `{"query":"query A { signup(input: {}) { user { id } } }", "operationName":"NonExistent"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})

		t.Run("multiple operations without operationName is rejected", func(t *testing.T) {
			body := `{"query":"query A { signup(input: {}) { user { id } } } query B { findByID(id: \"x\") { id } }"}`
			req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
			assert.NoError(t, err)

			result := isBypassed(req)
			assert.False(t, result)
		})
	})

	t.Run("should reject oversized request body", func(t *testing.T) {
		// Build a body that exceeds maxBypassBodySize (100 KB)
		padding := strings.Repeat("x", maxBypassBodySize+1)
		body := `{"query":"mutation { signup(input: {}) { user { id } } }", "extra":"` + padding + `"}`
		req, err := http.NewRequest(http.MethodPost, "/api/graphql", io.NopCloser(bytes.NewBufferString(body)))
		assert.NoError(t, err)

		result := isBypassed(req)
		assert.False(t, result)
	})
}
