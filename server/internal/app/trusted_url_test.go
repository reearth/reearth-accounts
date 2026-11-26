package app

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTrustedURL(t *testing.T) {
	accept := []string{
		"https://example.auth0.com/",
		"https://example.auth0.com./",
		"https://idp.example.com:8443/.well-known/jwks.json",
		"https://securetoken.google.com/my-proj",
		"https://EXAMPLE.COM",
	}
	for _, raw := range accept {
		t.Run("accept "+raw, func(t *testing.T) {
			assert.NoError(t, checkTrustedURL(raw))
		})
	}

	reject := []string{
		"http://example.auth0.com/",
		"HTTP://example.auth0.com/",
		"example.auth0.com",
		"//example.auth0.com/",
		"ftp://example.auth0.com/",
		"https://127.0.0.1/",
		"https://[::1]/",
		"https://[fe80::1%25en0]/",
		"https://[::ffff:127.0.0.1]/",
		"https://127.1/",
		"https://0x7f.1/",
		"https://2130706433/",
		"https://localhost/",
		"https://localhost./",
		"https://LOCALHOST/",
		"https://foo.localhost/",
		"https://0x7f.0x0.0x0.0x1/",
		"https://0x7f.0x1/",
		"https://127.0.0.0x1/",
		"https://169.254.169.0xfe/",
		"https://0177.0.0.01/",
		"https://app.localhost。/",
		"https://app.localhost．/",
		"https://auth。example。com/",
		"https://localhost.localdomain/",
		"https://x.localhost.localdomain/",
		"https://a.localhosｔ/",
		"https://a.LOCALHOSＴ/",
		"https://localhost.example.com/",
		"https://exa_mple.com/",
		"https://mock-oauth2/",
		"https://mock-oauth2:8080/default",
		"https://user:pass@example.auth0.com/",
		"https://a..example.com/",
		"https:///path",
		"https:example.com",
		"",
		"://bad",
	}
	for _, raw := range reject {
		t.Run("reject "+raw, func(t *testing.T) {
			assert.Error(t, checkTrustedURL(raw))
		})
	}
}

func TestCheckTrustedURL_ErrorOmitsCredentialsAndQuery(t *testing.T) {
	for _, raw := range []string{
		"https://alice:s3cr3t@example.auth0.com/?token=q-value",
		"http://alice:s3cr3t@example.auth0.com/?token=q-value",
		"https://alice:s3cr3t@127.0.0.1/?token=q-value",
		"alice:s3cr3t@example.auth0.com",
		"https://alice:s3cr3t@exa mple.com/?token=q-value",
	} {
		err := checkTrustedURL(raw)
		require.Error(t, err, raw)
		for _, leak := range []string{"alice", "s3cr3t", "q-value"} {
			assert.NotContains(t, err.Error(), leak, raw)
		}
	}
}

func TestCheckTrustedURLs(t *testing.T) {
	jwks := "http://10.0.0.1/jwks"
	emptyJWKS := ""
	providers := []appx.JWTProvider{
		{ISS: "https://example.auth0.com/"},
		{ISS: "https://idp.example.com/", JWKSURI: &jwks},
		{ISS: "http://mock-oauth2/"},
		{ISS: "https://ok.example.com/", JWKSURI: &emptyJWKS},
	}
	callers := ServiceCallers{
		{issuer: "https://localhost/", clientID: "svc-b"}:         {},
		{issuer: "https://example.auth0.com/", clientID: "svc-a"}: {},
	}

	err := checkTrustedURLs(providers, callers)
	require.Error(t, err)
	assert.Equal(t,
		"auth provider 2 jwks uri: scheme must be https\n"+
			"auth provider 3 issuer: scheme must be https\n"+
			`service caller "svc-b" issuer: host "localhost" is localhost`,
		err.Error())

	assert.NoError(t, checkTrustedURLs(nil, nil))
	assert.NoError(t, checkTrustedURLs(providers[:1], ServiceCallers{
		{issuer: "https://example.auth0.com/", clientID: "svc-a"}: {},
	}))
}

func TestReadConfig_TrustedURLs(t *testing.T) {
	providers := func(t *testing.T, ps ...appx.JWTProvider) string {
		t.Helper()
		b, err := json.Marshal(ps)
		require.NoError(t, err)
		return string(b)
	}
	str := func(s string) *string { return &s }
	callers := func(issuer string) string {
		return fmt.Sprintf(`[{"issuer":%q,"client_id":"svc","name":"svc","act_as":["integration"]}]`, issuer)
	}

	tests := []struct {
		name      string
		env       map[string]string
		debug     bool
		wantError string
	}{
		{
			name: "no auth providers",
			env:  map[string]string{"REEARTH_DEV": "false"},
		},
		{
			name: "https issuer",
			env: map[string]string{
				"REEARTH_DEV":  "false",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("https://idp.example.com/jwks")}),
			},
		},
		{
			name: "http issuer",
			env: map[string]string{
				"REEARTH_DEV":  "false",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "http://idp.example.com/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "http jwks uri",
			env: map[string]string{
				"REEARTH_DEV":  "false",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("http://idp.example.com/jwks")}),
			},
			wantError: "auth provider 1 jwks uri: scheme must be https",
		},
		{
			name: "IP jwks uri",
			env: map[string]string{
				"REEARTH_DEV":  "false",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("https://169.254.169.254/jwks")}),
			},
			wantError: `auth provider 1 jwks uri: host "169.254.169.254" is an IP address`,
		},
		{
			name: "service caller issuer",
			env: map[string]string{
				"REEARTH_DEV":                      "false",
				"REEARTH_AUTH":                     providers(t, appx.JWTProvider{ISS: "https://mock-oauth2/"}),
				"REEARTH_ACCOUNTS_SERVICE_CALLERS": callers("https://mock-oauth2/"),
			},
			wantError: `service caller "svc" issuer: host "mock-oauth2" is not a DNS name with a dot`,
		},
		{
			name: "auth0 domain over http",
			env: map[string]string{
				"REEARTH_DEV":          "false",
				"REEARTH_AUTH0_DOMAIN": "http://example.auth0.com",
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "auth issuer setting",
			env: map[string]string{
				"REEARTH_DEV":      "false",
				"REEARTH_AUTH_ISS": "https://localhost/",
			},
			wantError: `auth provider 1 issuer: host "localhost" is localhost`,
		},
		{
			name: "REEARTH_ACCOUNTS_DEV does not open dev mode",
			env: map[string]string{
				"REEARTH_DEV":          "false",
				"REEARTH_ACCOUNTS_DEV": "true",
				"REEARTH_AUTH":         providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "REEARTH_MOCK_AUTH does not open dev mode",
			env: map[string]string{
				"REEARTH_DEV":       "false",
				"REEARTH_MOCK_AUTH": "true",
				"REEARTH_AUTH":      providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "REEARTH_DEV opens dev mode",
			env: map[string]string{
				"REEARTH_DEV":                      "true",
				"REEARTH_AUTH":                     providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/", JWKSURI: str("http://127.0.0.1:8080/jwks")}),
				"REEARTH_ACCOUNTS_SERVICE_CALLERS": callers("http://mock-oauth2/"),
			},
		},
		{
			name:  "a non-release build opens dev mode",
			debug: true,
			env: map[string]string{
				"REEARTH_DEV":  "false",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SKIP_DOTENV", "1")
			for _, k := range []string{
				"REEARTH_AUTH0_DOMAIN", "REEARTH_AUTH_ISS",
				"REEARTH_ACCOUNTS_CIP_PROJECT_ID", "REEARTH_ACCOUNTS_SERVICE_CALLERS",
				"REEARTH_ACCOUNTS_DEV",
			} {
				t.Setenv(k, "")
			}
			t.Setenv("REEARTH_MOCK_AUTH", "false")
			t.Setenv("REEARTH_AUTH", "[]")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			c, err := ReadConfig(tt.debug)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.debug || tt.env["REEARTH_DEV"] == "true", c.Dev)
		})
	}
}
