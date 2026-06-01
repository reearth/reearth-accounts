package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/jarcoal/httpmock"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/idx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	jose "gopkg.in/go-jose/go-jose.v2"
)

// --- Mock_Auth=true (existing coverage) ---

func TestREST_AuthConfig(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/auth/config").Expect().Status(http.StatusOK).JSON().Object()
}

func TestREST_LogoutUnauthorized(t *testing.T) {
	cfg := &app.Config{} // no mock auth -> required auth must reject
	exp, _ := StartServer(t, cfg, false, nil)
	exp.POST("/api/auth/logout").Expect().Status(http.StatusUnauthorized)
}

func TestREST_LogoutWithMockAuth(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.POST("/api/auth/logout").Expect().Status(http.StatusOK).JSON().Object().ContainsKey("id")
}

// --- Mock_Auth=false (real JWT pipeline) ---
//
// The cases below exercise the real JWT pipeline (REEARTH_AUTH provider +
// appx.AuthMiddleware) end-to-end through the REST router — that's where most
// auth bugs actually surface in production. Each case spins up the server with
// a single test issuer (https://e2e.test.local/) whose JWKS is served via
// httpmock; requests to the in-process test server still go over real TCP via
// testRoundTripper so httpexpect can reach it.

const (
	jwtTestIssuer   = "https://e2e.test.local/"
	jwtTestJWKSURL  = "https://e2e.test.local/.well-known/jwks.json"
	jwtTestAudience = "https://accounts.test.local/api"
	jwtTestKeyID    = "test-kid-1"
)

// testRoundTripper routes requests to the test issuer through httpmock and
// every other request (including the in-process test server, regardless of
// whether it bound to 127.0.0.1, [::], or localhost) through the real
// transport. Mirrors the TestTransport pattern in appx/jwt_test.go.
type testRoundTripper struct {
	real          http.RoundTripper
	mockedHostSuf string // only hosts ending with this go through httpmock
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host != "" && strings.HasSuffix(host, t.mockedHostSuf) {
		return httpmock.DefaultTransport.RoundTrip(req)
	}
	return t.real.RoundTrip(req)
}

// installRealJWT activates httpmock with JWKS responders for the test issuer
// and installs a testRoundTripper that only sends test-issuer hosts through
// httpmock. It returns the RSA signing key plus a cleanup that restores the
// prior state; callers must defer the cleanup before the test exits.
//
// Order matters here: httpmock.Activate overwrites http.DefaultTransport with
// its own mock transport, so we capture the originally-installed transport via
// httpmock.InitialTransport AFTER activation and only then install our
// selective tripper.
func installRealJWT(t *testing.T) (*rsa.PrivateKey, func()) {
	t.Helper()

	httpmock.Activate()
	prev := httpmock.InitialTransport
	http.DefaultTransport = &testRoundTripper{real: prev, mockedHostSuf: "e2e.test.local"}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksBody, err := json.Marshal(jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{KeyID: jwtTestKeyID, Key: &key.PublicKey, Algorithm: jwt.SigningMethodRS256.Name, Use: "sig"},
		},
	})
	require.NoError(t, err)

	httpmock.RegisterResponder(http.MethodGet,
		jwtTestIssuer+".well-known/openid-configuration",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]string{
			"issuer":   jwtTestIssuer,
			"jwks_uri": jwtTestJWKSURL,
		}),
	)
	httpmock.RegisterResponder(http.MethodGet, jwtTestJWKSURL,
		httpmock.NewBytesResponder(http.StatusOK, jwksBody))

	return key, func() {
		httpmock.DeactivateAndReset() // restores http.DefaultTransport to InitialTransport
	}
}

// signTestToken builds an RS256-signed token with the test issuer + audience.
func signTestToken(t *testing.T, key *rsa.PrivateKey, sub string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":            jwtTestIssuer,
		"sub":            sub,
		"aud":            []string{jwtTestAudience},
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"name":           "Real JWT User",
		"email":          "jwt-user@example.com",
		"email_verified": true,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = jwtTestKeyID
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

// realAuthConfig builds an app.Config wired to the test issuer/audience with
// no mock auth, so the server actually runs appx.AuthMiddleware.
func realAuthConfig() *app.Config {
	jwks := jwtTestJWKSURL
	return &app.Config{
		Auth: app.AuthConfigs{
			appx.JWTProvider{
				ISS:     jwtTestIssuer,
				JWKSURI: &jwks,
				AUD:     []string{jwtTestAudience},
				ALG:     lo.ToPtr(jwt.SigningMethodRS256.Name),
			},
		},
	}
}

// seedJWTUser seeds a user whose Auth.Sub matches the given JWT subject so
// FindBySub resolves, plus the named roles (auto-seeded only under mock auth).
func seedJWTUser(sub string) Seeder {
	return func(ctx context.Context, r *repo.Container) error {
		for _, rt := range []role.RoleType{role.RoleOwner, role.RoleMaintainer, role.RoleWriter, role.RoleReader, role.RoleSelf} {
			rl := role.New().NewID().Name(string(rt)).MustBuild()
			if err := r.Role.Save(ctx, *rl); err != nil {
				return err
			}
		}

		uid := id.NewUserID()
		wid := id.NewWorkspaceID()

		u := user.New().ID(uid).
			Name("JWT User").
			Alias("jwt-user").
			Email("jwt-user@example.com").
			Auths([]user.Auth{user.AuthFrom(sub)}).
			Workspace(wid).
			MustBuild()
		if err := r.User.Save(ctx, u); err != nil {
			return err
		}

		ws := workspace.New().ID(wid).
			Name("JWT Personal").
			Alias("jwt-personal").
			Members(map[idx.ID[id.User]]workspace.Member{
				uid: {Role: role.RoleOwner, InvitedBy: uid},
			}).
			Personal(true).
			MustBuild()
		return r.Workspace.Save(ctx, ws)
	}
}

func TestREST_RealJWT_MissingToken(t *testing.T) {
	_, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, nil)

	// No Authorization header -> JWT middleware sees no token (credentials are
	// optional) and the resolver's nil AuthInfo => no user => RequiredAuth 401.
	exp.POST("/api/auth/logout").Expect().Status(http.StatusUnauthorized)
}

func TestREST_RealJWT_TamperedToken(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, nil)

	// Sign a valid token, then mutate the signature so JWKS verification fails.
	good := signTestToken(t, key, "test|signature-mismatch")
	parts := strings.Split(good, ".")
	require.Len(t, parts, 3)
	parts[2] = flipLastChar(parts[2])
	tampered := strings.Join(parts, ".")

	// Tampered signature -> the JWT middleware itself rejects the request before
	// it reaches the resolver. The middleware returns 401.
	exp.POST("/api/auth/logout").
		WithHeader("Authorization", "Bearer "+tampered).
		Expect().Status(http.StatusUnauthorized)
}

func TestREST_RealJWT_ValidTokenReachesResolver_UserNotFound(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	// No seed -> token validates cryptographically but FindBySub returns
	// not-found, so RequiredAuth answers 401. The fact we get a 401 from THIS
	// branch (not from JWT rejection) is the positive validation signal: the
	// bearer token successfully cleared the middleware.
	exp, _ := StartServer(t, realAuthConfig(), false, nil)

	token := signTestToken(t, key, "test|unknown-user")
	exp.POST("/api/auth/logout").
		WithHeader("Authorization", "Bearer "+token).
		Expect().Status(http.StatusUnauthorized)
}

func TestREST_RealJWT_ValidTokenResolvesUser(t *testing.T) {
	const sub = "test|known-user"

	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUser(sub))

	token := signTestToken(t, key, sub)

	// /api/auth/config is public; sanity-check it under non-mock auth.
	exp.GET("/api/auth/config").Expect().Status(http.StatusOK)

	// Authenticated endpoints: token validates, resolver finds the seeded user,
	// /api/auth/logout returns the user payload and /api/users/me returns the
	// authenticated identity.
	exp.POST("/api/auth/logout").
		WithHeader("Authorization", "Bearer "+token).
		Expect().Status(http.StatusOK).
		JSON().Object().HasValue("name", "JWT User")

	exp.GET("/api/users/me").
		WithHeader("Authorization", "Bearer "+token).
		Expect().Status(http.StatusOK).
		JSON().Object().HasValue("email", "jwt-user@example.com")
}

// flipLastChar mutates the last character of a base64-url segment so the
// signature no longer matches, producing a token the JWKS-backed verifier
// rejects without changing its structural validity.
func flipLastChar(s string) string {
	if s == "" {
		return "A"
	}
	last := s[len(s)-1]
	if last == 'A' {
		return s[:len(s)-1] + "B"
	}
	return s[:len(s)-1] + "A"
}
