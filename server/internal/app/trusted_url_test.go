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

func TestCheckLocalURL(t *testing.T) {
	accept := []string{
		"http://127.0.0.1:8080/jwks",
		"https://127.0.0.1/",
		"http://127.2.3.4/",
		"http://[::1]:8080/",
		"http://[::ffff:127.0.0.1]/",
		"http://localhost:8080/",
		"http://LOCALHOST./",
		"http://mock-oauth2/",
		"http://mock-oauth2:8080/default",
		"https://Keycloak/realms/dev",
		"HTTP://mock-oauth2/",
	}
	for _, raw := range accept {
		t.Run("accept "+raw, func(t *testing.T) {
			assert.NoError(t, checkLocalURL(raw))
		})
	}

	reject := []string{
		"http://idp.example.com/",
		"https://10.0.0.1/",
		"http://192.168.1.1/",
		"http://169.254.169.254/",
		"http://[fe80::1]/",
		"http://0.0.0.0/",
		"http://[::]/",
		"http://127.1/",
		"http://0x7f000001/",
		"http://2130706433/",
		"http://foo.localhost/",
		"http://metadata/computeMetadata/v1/",
		"http://METADATA/",
		"http://instance-data/latest/meta-data/",
		"http://localhost.localdomain/",
		"http://mock-oauth2./",
		"http://1mock/",
		"http://-mock/",
		"http://mock_oauth2/",
		"http://mocｋ/",
		"http://mōck/",
		"ftp://localhost/",
		"localhost",
		"http://user:pass@localhost/",
		"http:///path",
		"",
		"://bad",
	}
	for _, raw := range reject {
		t.Run("reject "+raw, func(t *testing.T) {
			assert.Error(t, checkLocalURL(raw))
		})
	}
}

func TestCheckTrustedURLs(t *testing.T) {
	jwks := "http://10.0.0.1/jwks"
	localJWKS := "http://127.0.0.1:8080/jwks"
	emptyJWKS := ""
	providers := []appx.JWTProvider{
		{ISS: "https://example.auth0.com/"},
		{ISS: "https://idp.example.com/", JWKSURI: &jwks},
		{ISS: "http://mock-oauth2/", JWKSURI: &localJWKS},
		{ISS: "https://ok.example.com/", JWKSURI: &emptyJWKS},
		{ISS: "http://idp.example.com/"},
	}
	callers := ServiceCallers{
		{issuer: "https://localhost/", clientID: "svc-b"}:         {},
		{issuer: "https://example.auth0.com/", clientID: "svc-a"}: {},
	}
	hint := " (on a developer machine only, REEARTH_ACCOUNTS_LOCAL_ISSUER=true allows loopback and single-label hosts)"

	err := checkTrustedURLs(providers, callers, false)
	require.Error(t, err)
	assert.Equal(t,
		"auth provider 2 jwks uri: scheme must be https\n"+
			"auth provider 3 issuer: scheme must be https"+hint+"\n"+
			"auth provider 3 jwks uri: scheme must be https"+hint+"\n"+
			"auth provider 5 issuer: scheme must be https\n"+
			`service caller "svc-b" issuer: host "localhost" is localhost`+hint,
		err.Error())
	assert.NotContains(t, err.Error(), "REEARTH_DEV")

	err = checkTrustedURLs(providers, callers, true)
	require.Error(t, err)
	assert.Equal(t,
		"auth provider 2 jwks uri: scheme must be https\n"+
			"auth provider 5 issuer: scheme must be https",
		err.Error())

	assert.NoError(t, checkTrustedURLs(nil, nil, false))
	assert.NoError(t, checkTrustedURLs(providers[:1], ServiceCallers{
		{issuer: "https://example.auth0.com/", clientID: "svc-a"}: {},
	}, false))
	assert.NoError(t, checkTrustedURLs(providers[2:4], callers, true))
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
		name            string
		env             map[string]string
		debug           bool
		wantError       string
		wantLocalIssuer bool
		wantDev         bool
	}{
		{
			name: "no auth providers",
		},
		{
			name: "https issuer",
			env: map[string]string{
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("https://idp.example.com/jwks")}),
			},
		},
		{
			name: "http issuer",
			env: map[string]string{
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "http://idp.example.com/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "http jwks uri",
			env: map[string]string{
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("http://idp.example.com/jwks")}),
			},
			wantError: "auth provider 1 jwks uri: scheme must be https",
		},
		{
			name: "IP jwks uri",
			env: map[string]string{
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/", JWKSURI: str("https://169.254.169.254/jwks")}),
			},
			wantError: `auth provider 1 jwks uri: host "169.254.169.254" is an IP address`,
		},
		{
			name: "service caller issuer",
			env: map[string]string{
				"REEARTH_AUTH":                     providers(t, appx.JWTProvider{ISS: "https://mock-oauth2/"}),
				"REEARTH_ACCOUNTS_SERVICE_CALLERS": callers("https://mock-oauth2/"),
			},
			wantError: `service caller "svc" issuer: host "mock-oauth2" is not a DNS name with a dot`,
		},
		{
			name: "auth0 domain over http",
			env: map[string]string{
				"REEARTH_AUTH0_DOMAIN": "http://example.auth0.com",
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "auth issuer setting",
			env: map[string]string{
				"REEARTH_AUTH_ISS": "https://localhost/",
			},
			wantError: `auth provider 1 issuer: host "localhost" is localhost`,
		},
		{
			name: "REEARTH_ACCOUNTS_DEV does not turn the check off",
			env: map[string]string{
				"REEARTH_ACCOUNTS_DEV": "true",
				"REEARTH_AUTH":         providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https (on a developer machine only, REEARTH_ACCOUNTS_LOCAL_ISSUER=true",
		},
		{
			name: "REEARTH_MOCK_AUTH does not turn the check off",
			env: map[string]string{
				"REEARTH_MOCK_AUTH": "true",
				"REEARTH_AUTH":      providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "REEARTH_DEV does not turn the check off",
			env: map[string]string{
				"REEARTH_DEV":                      "true",
				"REEARTH_AUTH":                     providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/", JWKSURI: str("http://127.0.0.1:8080/jwks")}),
				"REEARTH_ACCOUNTS_SERVICE_CALLERS": callers("http://mock-oauth2/"),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name:  "a non-release build does not turn the check off",
			debug: true,
			env: map[string]string{
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "local issuer switch accepts a local issuer",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER":    "true",
				"REEARTH_AUTH":                     providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/", JWKSURI: str("http://127.0.0.1:8080/jwks")}),
				"REEARTH_ACCOUNTS_SERVICE_CALLERS": callers("http://mock-oauth2/"),
			},
			wantLocalIssuer: true,
		},
		{
			name: "a prefixed spelling of the switch does not turn it on",
			env: map[string]string{
				"REEARTH_REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"REEARTH_AUTH":                          providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "local issuer switch with a value that is not a boolean",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "yes please",
			},
			wantError: "REEARTH_ACCOUNTS_LOCAL_ISSUER: not a boolean",
		},
		{
			name: "local issuer switch is refused on Cloud Run",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"K_SERVICE":                     "accounts",
			},
			wantError: "must not be set on Cloud Run",
		},
		{
			name: "local issuer switch is refused on Kubernetes",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"KUBERNETES_SERVICE_HOST":       "10.0.0.1",
			},
			wantError: "must not be set on Kubernetes",
		},
		{
			name: "a managed platform without the switch starts",
			env: map[string]string{
				"K_SERVICE":    "accounts",
				"REEARTH_AUTH": providers(t, appx.JWTProvider{ISS: "https://idp.example.com/"}),
			},
		},
		{
			name:  "local issuer switch with dev mode",
			debug: true,
			env: map[string]string{
				"REEARTH_DEV":                   "true",
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"REEARTH_AUTH":                  providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/"}),
			},
			wantLocalIssuer: true,
			wantDev:         true,
		},
		{
			name: "local issuer switch still refuses an http DNS name",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"REEARTH_AUTH":                  providers(t, appx.JWTProvider{ISS: "http://idp.example.com/"}),
			},
			wantError: "auth provider 1 issuer: scheme must be https",
		},
		{
			name: "local issuer switch still refuses a private IP jwks uri",
			env: map[string]string{
				"REEARTH_ACCOUNTS_LOCAL_ISSUER": "true",
				"REEARTH_AUTH":                  providers(t, appx.JWTProvider{ISS: "http://mock-oauth2/", JWKSURI: str("https://10.0.0.1/jwks")}),
			},
			wantError: `auth provider 1 jwks uri: host "10.0.0.1" is an IP address`,
		},
		{
			name:    "REEARTH_DEV still sets dev mode",
			env:     map[string]string{"REEARTH_DEV": "true"},
			wantDev: true,
		},
		{
			name:    "a non-release build still sets dev mode",
			debug:   true,
			wantDev: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SKIP_DOTENV", "1")
			for _, k := range []string{
				"REEARTH_AUTH0_DOMAIN", "REEARTH_AUTH_ISS",
				"REEARTH_ACCOUNTS_CIP_PROJECT_ID", "REEARTH_ACCOUNTS_SERVICE_CALLERS",
				"REEARTH_ACCOUNTS_DEV", "K_SERVICE", "KUBERNETES_SERVICE_HOST",
			} {
				t.Setenv(k, "")
			}
			for _, k := range []string{"REEARTH_DEV", "REEARTH_MOCK_AUTH", "REEARTH_ACCOUNTS_LOCAL_ISSUER"} {
				t.Setenv(k, "false")
			}
			t.Setenv("REEARTH_AUTH", "[]")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			c, err := ReadConfig(tt.debug)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.NotContains(t, err.Error(), "REEARTH_DEV")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLocalIssuer, c.LocalIssuer)
			assert.Equal(t, tt.wantDev, c.Dev)
		})
	}
}
