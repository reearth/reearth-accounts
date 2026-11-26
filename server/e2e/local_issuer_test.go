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

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/pkg/servicetoken"
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
)

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

func authorizationCodeToken(t *testing.T, issuer string) string {
	t.Helper()
	const redirect = "http://localhost/callback"
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(issuer + "/authorize?" + url.Values{
		"client_id": {"local-web"}, "response_type": {"code"}, "redirect_uri": {redirect},
		"scope": {"openid"}, "state": {"s"}, "nonce": {"n"},
	}.Encode())
	require.NoError(t, err)
	_ = res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)

	res, err = http.PostForm(issuer+"/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"local-web"},
		"redirect_uri": {redirect}, "audience": {localAudience},
	})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tr))
	return tr.AccessToken
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
	host := startServerURL(t, cfg, seededMemoryRepos(t, seedServiceCallers), nil, false)

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

	t.Run("user token from the mock is not a client token", func(t *testing.T) {
		user := authorizationCodeToken(t, issuer)
		c := jwtClaims(t, user)
		assert.Equal(t, issuer, c["iss"])
		assert.Equal(t, localAudience, c["aud"])
		assert.NotEmpty(t, c["email"])
		assert.NotContains(t, c, "gty")

		status, body := postGraphQL(t, host, user, asIntegration, byUser)
		assert.Equal(t, http.StatusUnauthorized, status, body)
		assert.NotContains(t, body, "JWT is invalid", "the token validates, and the act-as header is refused because it is not a client token")
	})
}
