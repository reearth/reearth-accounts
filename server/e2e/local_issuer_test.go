//go:build integration

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/servicetoken"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	localIssuerID       = "accounts-local"
	localAudience       = "reearth-accounts-local"
	localClientID       = "cms"
	localIssuerPortSpec = "8091/tcp"
	localUserSub        = "local-user"
	chosenUserSub       = "chosen-user"
)

var localUID = id.NewUserID()

func seedLocalIssuer(ctx context.Context, r *repo.Container) error {
	if err := seedServiceCallers(ctx, r); err != nil {
		return err
	}
	u := user.New().ID(localUID).Name(localUserSub).Email("local-user@e2e.com").Auths([]user.Auth{user.AuthFrom(localUserSub)}).MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}
	return r.Workspace.Save(ctx, workspace.New().NewID().Name(localUserSub).Alias("a-"+localUID.String()).Personal(true).
		Members(map[workspace.UserID]workspace.Member{localUID: {Role: role.RoleOwner, InvitedBy: localUID}}).MustBuild())
}

func mockIssuerImage(t *testing.T) string {
	t.Helper()
	compose, err := os.ReadFile("../../docker-compose.dev.yml")
	require.NoError(t, err)
	m := regexp.MustCompile(`image: (ghcr\.io/navikt/mock-oauth2-server:\S+@sha256:[0-9a-f]{64})`).FindSubmatch(compose)
	require.NotNil(t, m, "docker-compose.dev.yml pins the mock issuer by version and digest")
	return string(m[1])
}

func startMockIssuer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.Run(ctx, mockIssuerImage(t),
		testcontainers.WithEnv(map[string]string{"SERVER_PORT": "8091", "JSON_CONFIG_PATH": "/config/config.json"}),
		testcontainers.WithExposedPorts(localIssuerPortSpec),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      "../dev/mock-oauth2/config.json",
			ContainerFilePath: "/config/config.json",
			FileMode:          0o444,
		}),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/isalive").WithPort(localIssuerPortSpec)),
	)
	testcontainers.CleanupContainer(t, c)
	require.NoError(t, err)
	p, err := c.MappedPort(ctx, localIssuerPortSpec)
	require.NoError(t, err)
	return p.Port()
}

func localIssuerEnv(t *testing.T, issuer string) {
	t.Helper()
	auth, err := json.Marshal([]map[string]any{{"iss": issuer, "aud": []string{localAudience}}})
	require.NoError(t, err)
	callers, err := json.Marshal([]map[string]any{{"issuer": issuer, "client_id": localClientID, "name": localClientID, "act_as": []string{"integration"}}})
	require.NoError(t, err)
	t.Setenv("SKIP_DOTENV", "1")
	t.Setenv("K_SERVICE", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("REEARTH_ACCOUNTS_LOCAL_ISSUER", "")
	t.Setenv("REEARTH_AUTH", string(auth))
	t.Setenv("REEARTH_ACCOUNTS_SERVICE_CALLERS", string(callers))
}

func clientCredentialsToken(t *testing.T, tokenURL string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	src, err := servicetoken.New(servicetoken.Config{TokenURL: tokenURL, ClientID: localClientID, Audience: localAudience, Key: key})
	require.NoError(t, err)
	tok, err := src.Token(context.Background())
	require.NoError(t, err)
	return tok
}

func tokenResponse(t *testing.T, issuer string, form url.Values, basicAuthClient string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, issuer+"/token", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuthClient != "" {
		req.SetBasicAuth(basicAuthClient, "any")
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, string(b))
	var tr map[string]any
	require.NoError(t, json.Unmarshal(b, &tr))
	return tr
}

func authorizationCode(t *testing.T, issuer string) map[string]any {
	t.Helper()
	const redirect = "http://localhost/callback"
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(issuer + "/authorize?" + url.Values{
		"client_id": {"local-web"}, "response_type": {"code"}, "redirect_uri": {redirect},
		"scope": {"openid offline_access"}, "state": {"s"}, "nonce": {"n"},
	}.Encode())
	require.NoError(t, err)
	_ = res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)

	return tokenResponse(t, issuer, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"local-web"},
		"redirect_uri": {redirect}, "audience": {localAudience},
	}, "")
}

func authorizationCodeToken(t *testing.T, issuer string) string {
	t.Helper()
	return authorizationCode(t, issuer)["access_token"].(string)
}

func selfSignedJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func jwtClaims(t *testing.T, tok string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var c map[string]any
	require.NoError(t, json.Unmarshal(payload, &c))
	return c
}

func postGraphQL(t *testing.T, host, token, actAs, query string) (int, string) {
	t.Helper()
	body, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, host+"/api/graphql", strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if actAs != "" {
		req.Header.Set(app.ActAsHeader, actAs)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, string(b)
}

func TestLocalIssuer(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	port := startMockIssuer(t)
	issuer := "http://localhost:" + port + "/" + localIssuerID
	localIssuerEnv(t, issuer)

	_, err := app.ReadConfig(false)
	require.ErrorContains(t, err, "untrusted token URL", "the mock issuer is refused without the local issuer switch")

	t.Setenv("REEARTH_ACCOUNTS_LOCAL_ISSUER", "true")
	cfg, err := app.ReadConfig(false)
	require.NoError(t, err)
	host := startServerURL(t, cfg, seededMemoryRepos(t, seedLocalIssuer), nil, false)

	asIntegration := "integration:" + svcIntegration.String()
	byUser := `{ findByUser(userId: "` + svcOwnerUID.String() + `") { id } }`
	tok := clientCredentialsToken(t, issuer+"/token")

	t.Run("client token has a deployed issuer's shape", func(t *testing.T) {
		c := jwtClaims(t, tok)
		assert.Equal(t, issuer, c["iss"])
		assert.Equal(t, localClientID+"@clients", c["sub"])
		assert.Equal(t, localClientID, c["azp"])
		assert.Equal(t, "client-credentials", c["gty"])
		assert.Equal(t, localAudience, c["aud"])
		assert.NotContains(t, c, "email")
		iat, exp := c["iat"].(float64), c["exp"].(float64)
		assert.Greater(t, exp, iat)
		assert.LessOrEqual(t, exp-iat, float64(3600))
	})

	t.Run("client acting for an integration reads its workspaces", func(t *testing.T) {
		status, body := postGraphQL(t, host, tok, asIntegration, byUser)
		require.Equal(t, http.StatusOK, status, body)
		var res struct {
			Data struct {
				FindByUser []struct{ ID string } `json:"findByUser"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &res), body)
		var ids []string
		for _, w := range res.Data.FindByUser {
			ids = append(ids, w.ID)
		}
		assert.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, ids)
	})

	t.Run("token fetched through another hostname is refused", func(t *testing.T) {
		other := clientCredentialsToken(t, "http://127.0.0.1:"+port+"/"+localIssuerID+"/token")
		assert.Equal(t, "http://127.0.0.1:"+port+"/"+localIssuerID, jwtClaims(t, other)["iss"])
		status, body := postGraphQL(t, host, other, asIntegration, byUser)
		assert.Equal(t, http.StatusUnauthorized, status, body)
	})

	me := `{ me { id } }`
	meID := func(t *testing.T, body string) string {
		t.Helper()
		var res struct {
			Data struct {
				Me struct{ ID string } `json:"me"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &res), body)
		return res.Data.Me.ID
	}

	t.Run("user token from the mock is not a client token", func(t *testing.T) {
		user := authorizationCodeToken(t, issuer)
		c := jwtClaims(t, user)
		assert.Equal(t, issuer, c["iss"])
		assert.Equal(t, localUserSub, c["sub"])
		assert.Equal(t, localAudience, c["aud"])
		assert.NotEmpty(t, c["email"])
		assert.NotContains(t, c, "gty")

		status, body := postGraphQL(t, host, user, "", me)
		require.Equal(t, http.StatusOK, status, body)
		assert.Equal(t, localUID.String(), meID(t, body))

		status, body = postGraphQL(t, host, user, asIntegration, byUser)
		assert.Equal(t, http.StatusUnauthorized, status, body)
	})

	t.Run("grant types the config does not map never give a client call", func(t *testing.T) {
		code := authorizationCode(t, issuer)
		require.NotEmpty(t, code["refresh_token"])
		chosen := selfSignedJWT(t, jwt.MapClaims{"sub": chosenUserSub, "aud": localAudience, "iss": "anyone", "exp": 4102444800})
		password := func(username string) url.Values {
			return url.Values{"grant_type": {"password"}, "username": {username}, "password": {"x"}, "client_id": {"local-web"}, "scope": {"openid"}, "audience": {localAudience}}
		}

		cases := []struct {
			name      string
			form      url.Values
			basicAuth string
			sub       any
		}{
			{"password", password(chosenUserSub), "", chosenUserSub},
			{"password imitating a client", password(localClientID + "@clients"), "", localClientID + "@clients"},
			{"jwt-bearer", url.Values{
				"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {chosen},
				"client_id": {localClientID}, "scope": {"openid"}, "audience": {localAudience},
			}, "", chosenUserSub},
			{"token exchange", url.Values{
				"grant_type": {"urn:ietf:params:oauth:grant-type:token-exchange"}, "subject_token": {chosen},
				"subject_token_type": {"urn:ietf:params:oauth:token-type:jwt"}, "audience": {localAudience},
			}, localClientID, chosenUserSub},
			{"refresh_token", url.Values{
				"grant_type": {"refresh_token"}, "refresh_token": {code["refresh_token"].(string)}, "client_id": {"local-web"},
			}, "", nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tok := tokenResponse(t, issuer, tc.form, tc.basicAuth)["access_token"].(string)
				c := jwtClaims(t, tok)
				assert.Equal(t, issuer, c["iss"])
				assert.Equal(t, tc.sub, c["sub"])
				for _, claim := range []string{"aud", "azp", "gty", "client_id", "email"} {
					assert.NotContains(t, c, claim)
				}

				status, body := postGraphQL(t, host, tok, asIntegration, byUser)
				assert.Equal(t, http.StatusUnauthorized, status, body)
				status, body = postGraphQL(t, host, tok, "", me)
				assert.Equal(t, http.StatusUnauthorized, status, body)
			})
		}
	})
}
