package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/jarcoal/httpmock"
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

	t.Run("signupOIDC with a body identity records nothing", func(t *testing.T) {
		signupOIDCGQL(e, "oidc@example.com", policyList("tos", tosCurrent, "privacy", privacyCurrent)).NotContainsKey("errors")
		uid := signedUp(t, r, "oidc@example.com")
		pendingOf(e, uid.String()).Length().IsEqual(2)
		assert.Empty(t, storedAcceptances(t, r, uid))
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

func TestPolicySignup_OIDCRecordsPoliciesOnlyForIssuerIdentity(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testPolicySignupOIDCFromIssuer(t, useMongo)
		})
	}
}

func testPolicySignupOIDCFromIssuer(t *testing.T, useMongo bool) {
	key, cleanup := installRealJWT(t)
	defer cleanup()
	const sub, email = "auth0|issuer-user", "issuer-user@example.com"
	httpmock.RegisterResponder(http.MethodGet, jwtTestIssuer+".well-known/openid-configuration",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]string{
			"issuer":            jwtTestIssuer,
			"jwks_uri":          jwtTestJWKSURL,
			"userinfo_endpoint": jwtTestIssuer + "userinfo",
		}))
	httpmock.RegisterResponder(http.MethodGet, jwtTestIssuer+"userinfo",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]string{"sub": sub, "email": email, "name": "issuer-user"}))

	cfg := realAuthConfig()
	cfg.PolicyWebhookSecret = webhookSecret
	now := time.Now().Truncate(time.Second)
	e, r := StartServer(t, cfg, useMongo, seedRoles)
	deliverPolicies(t, e, currentTosPayload(now, -2*time.Minute))
	deliverPolicies(t, e, upcomingTosPayload(now))

	e.POST("/api/users/signup-oidc").
		WithHeader("Authorization", "Bearer "+signTestToken(t, key, sub)).
		WithHeader("Origin", policyOrigin).
		WithHeader("User-Agent", policyUserAgent).
		WithJSON(map[string]any{"policies": policyList("tos", tosUpcoming, "privacy", privacyCurrent)}).
		Expect().Status(http.StatusOK)

	uid := signedUp(t, r, email)
	as := storedAcceptances(t, r, uid)
	require.Len(t, as, 2)
	for _, a := range as {
		assert.Equal(t, uid, a.UserID)
		assert.Equal(t, policyUserAgent, a.Client.UserAgent)
	}
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

func TestSignup_RejectsAnIDAlreadyInUse(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testSignupRejectsAnIDAlreadyInUse(t, useMongo)
		})
	}
}

func testSignupRejectsAnIDAlreadyInUse(t *testing.T, useMongo bool) {
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	e, r := StartServer(t, &app.Config{PolicyWebhookSecret: webhookSecret}, useMongo, seedRoles)
	deliverPolicies(t, e, currentTosPayload(now, -2*time.Minute))

	signupGQL(e, "existing@example.com", nil).NotContainsKey("errors")
	existing := signedUp(t, r, "existing@example.com")
	policies := policyList("tos", tosCurrent, "privacy", privacyCurrent)

	gqlIn := func(email string) map[string]any {
		return map[string]any{"id": existing.String(), "email": email, "name": email, "policies": policies}
	}
	in := gqlIn("gql@example.com")
	in["password"] = "StrongPassw0rd!"
	e.POST("/api/graphql").
		WithJSON(GraphQLRequest{Query: signupMutation, Variables: map[string]any{"input": in}}).
		Expect().Status(http.StatusOK).JSON().Object().Value("errors").Array().NotEmpty()

	in = gqlIn("oidc@example.com")
	in["sub"] = "auth0|oidc"
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-Auth-Sub", "auth0|oidc").
		WithHeader("X-Reearth-Debug-Auth-Iss", "https://issuer.example.com").
		WithHeader("X-Reearth-Debug-Auth-Token", "dummy").
		WithJSON(GraphQLRequest{Query: signupOIDCMutation, Variables: map[string]any{"input": in}}).
		Expect().Status(http.StatusOK).JSON().Object().Value("errors").Array().NotEmpty()

	e.POST("/api/users/signup").
		WithJSON(map[string]any{"id": existing.String(), "email": "rest@example.com", "name": "rest", "password": "StrongPassw0rd!", "policies": policies}).
		Expect().Status(http.StatusConflict)

	e.POST("/api/users/signup-oidc").
		WithJSON(map[string]any{"id": existing.String(), "email": "rest-oidc@example.com", "name": "rest-oidc", "sub": "auth0|rest-oidc", "policies": policies}).
		Expect().Status(http.StatusConflict)

	for _, email := range []string{"gql@example.com", "oidc@example.com", "rest@example.com", "rest-oidc@example.com"} {
		assertNotSignedUp(t, r, email)
	}
	assert.Empty(t, storedAcceptances(t, r, existing))
	ws, err := r.Workspace.FindByUser(ctx, existing)
	require.NoError(t, err)
	assert.Len(t, ws, 1)
	u, err := r.User.FindByID(ctx, existing)
	require.NoError(t, err)
	assert.Equal(t, "existing@example.com", u.Email())
}
