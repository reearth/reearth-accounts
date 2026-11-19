package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tosStale = "0.1.0"

	signupMutation     = `mutation($input: SignupInput!) { signup(input: $input) { user { id } } }`
	signupOIDCMutation = `mutation($input: SignupOIDCInput!) { signupOIDC(input: $input) { user { id } } }`
)

func policyList(refs ...string) []map[string]string {
	return acceptInput(refs...)["input"].([]map[string]string)
}

func signupGQL(e *httpexpect.Expect, email string, policies []map[string]string) *httpexpect.Object {
	in := map[string]any{"email": email, "name": email, "password": "StrongPassw0rd!"}
	if policies != nil {
		in["policies"] = policies
	}
	return e.POST("/api/graphql").
		WithHeader("Origin", policyOrigin).
		WithHeader("User-Agent", policyUserAgent).
		WithHeader("X-Forwarded-For", forgedIP).
		WithJSON(GraphQLRequest{Query: signupMutation, Variables: map[string]any{"input": in}}).
		Expect().Status(http.StatusOK).JSON().Object()
}

func signupOIDCGQL(e *httpexpect.Expect, email string, policies []map[string]string) *httpexpect.Object {
	in := map[string]any{"email": email, "name": email, "sub": "auth0|" + email, "policies": policies}
	return e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Origin", policyOrigin).
		WithHeader("User-Agent", policyUserAgent).
		WithHeader("X-Forwarded-For", forgedIP).
		WithHeader("X-Reearth-Debug-Auth-Sub", "auth0|"+email).
		WithHeader("X-Reearth-Debug-Auth-Iss", "https://issuer.example.com").
		WithHeader("X-Reearth-Debug-Auth-Token", "dummy").
		WithJSON(GraphQLRequest{Query: signupOIDCMutation, Variables: map[string]any{"input": in}}).
		Expect().Status(http.StatusOK).JSON().Object()
}

func signupREST(e *httpexpect.Expect, email string, policies []map[string]string) *httpexpect.Response {
	return demoUserREST(e.POST("/api/users/signup")).
		WithJSON(map[string]any{"email": email, "name": email, "password": "StrongPassw0rd!", "policies": policies}).
		Expect()
}

func pendingOf(e *httpexpect.Expect, uid string) *httpexpect.Array {
	return e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uid).
		WithJSON(GraphQLRequest{Query: `{ me { pendingPolicies { type } } }`}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("data").Object().Value("me").Object().Value("pendingPolicies").Array()
}

func signedUp(t *testing.T, r *repo.Container, email string) id.UserID {
	t.Helper()
	u, err := r.User.FindByEmail(context.Background(), email)
	require.NoError(t, err)
	return u.ID()
}

func assertNotSignedUp(t *testing.T, r *repo.Container, email string) {
	t.Helper()
	_, err := r.User.FindByEmail(context.Background(), email)
	assert.ErrorIs(t, err, rerror.ErrNotFound, email)
}

func TestPolicySignup(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testPolicySignup(t, useMongo)
		})
	}
}

func testPolicySignup(t *testing.T, useMongo bool) {
	now := time.Now().Truncate(time.Second)
	e, r := StartServer(t, &app.Config{PolicyWebhookSecret: webhookSecret}, useMongo, seedRoles)
	deliverPolicies(t, e, currentTosPayload(now, -2*time.Minute))
	deliverPolicies(t, e, upcomingTosPayload(now))

	t.Run("signup records the client", func(t *testing.T) {
		signupGQL(e, "gql@example.com", policyList("tos", tosCurrent, "privacy", privacyCurrent)).NotContainsKey("errors")
		uid := signedUp(t, r, "gql@example.com")
		pendingOf(e, uid.String()).IsEmpty()

		as := storedAcceptances(t, r, uid)
		require.Len(t, as, 2)
		for _, a := range as {
			assert.Equal(t, policyOrigin, a.Client.Origin)
			assert.Equal(t, policyUserAgent, a.Client.UserAgent)
			assert.True(t, a.Client.IP.IsLoopback(), a.Client.IP)
		}
	})

	t.Run("signupOIDC with the upcoming version", func(t *testing.T) {
		signupOIDCGQL(e, "oidc@example.com", policyList("tos", tosUpcoming, "privacy", privacyCurrent)).NotContainsKey("errors")
		uid := signedUp(t, r, "oidc@example.com")
		pendingOf(e, uid.String()).IsEmpty()
		assert.Len(t, storedAcceptances(t, r, uid), 2)
	})

	t.Run("REST signup", func(t *testing.T) {
		signupREST(e, "rest@example.com", policyList("tos", tosCurrent, "privacy", privacyCurrent)).Status(http.StatusOK)
		uid := signedUp(t, r, "rest@example.com")
		pendingOf(e, uid.String()).IsEmpty()
		assert.Len(t, storedAcceptances(t, r, uid), 2)
	})

	t.Run("signup without policies leaves them pending", func(t *testing.T) {
		signupGQL(e, "plain@example.com", nil).NotContainsKey("errors")
		pendingOf(e, signedUp(t, r, "plain@example.com").String()).Length().IsEqual(2)
	})

	t.Run("a refused version creates no user", func(t *testing.T) {
		for name, policies := range map[string][]map[string]string{
			"stale":     policyList("tos", tosStale),
			"unknown":   policyList("cookies", tosCurrent),
			"malformed": policyList("tos", malformedVersion),
		} {
			signupGQL(e, "gql-"+name+"@example.com", policies).Value("errors").Array().NotEmpty()
			signupOIDCGQL(e, "oidc-"+name+"@example.com", policies).Value("errors").Array().NotEmpty()
			signupREST(e, "rest-"+name+"@example.com", policies).Status(http.StatusBadRequest)
			for _, prefix := range []string{"gql-", "oidc-", "rest-"} {
				assertNotSignedUp(t, r, prefix+name+"@example.com")
			}
		}
		signupGQL(e, "stale@example.com", policyList("tos", tosStale)).Value("errors").Array().
			Value(0).Object().Value("message").String().Contains("neither current nor upcoming")
	})
}

func TestPolicySignup_WithoutSecret(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, false, seedRoles)

	signupGQL(e, "gql@example.com", policyList("tos", tosCurrent)).Value("errors").Array().
		Value(0).Object().Value("message").String().Contains("policies not configured")
	signupOIDCGQL(e, "oidc@example.com", policyList("tos", tosCurrent)).Value("errors").Array().
		Value(0).Object().Value("message").String().Contains("policies not configured")
	signupREST(e, "rest@example.com", policyList("tos", tosCurrent)).Status(http.StatusBadRequest).
		JSON().Object().Value("message").String().Contains("policies not configured")
	for _, email := range []string{"gql@example.com", "oidc@example.com", "rest@example.com"} {
		assertNotSignedUp(t, r, email)
	}

	signupGQL(e, "empty@example.com", policyList()).NotContainsKey("errors")
	signupOIDCGQL(e, "empty-oidc@example.com", nil).NotContainsKey("errors")
	signupREST(e, "empty-rest@example.com", nil).Status(http.StatusOK)
	signedUp(t, r, "empty@example.com")
}
