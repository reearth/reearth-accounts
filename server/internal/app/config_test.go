package app

import (
	"testing"
	"time"

	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig_MFARecoveryMaxAuthAge(t *testing.T) {
	t.Setenv("SKIP_DOTENV", "1")

	t.Setenv("REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE", "5m")
	c, err := ReadConfig(false)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, c.MFARecoveryMaxAuthAge)

	t.Setenv("REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE", "-1m")
	_, err = ReadConfig(false)
	assert.EqualError(t, err, "REEARTH_ACCOUNTS_MFA_RECOVERY_MAX_AUTH_AGE must not be negative, got -1m0s")
}

func TestReadConfig_AuthTimeClaim(t *testing.T) {
	t.Setenv("SKIP_DOTENV", "1")

	t.Setenv("REEARTH_ACCOUNTS_AUTH_TIME_CLAIM", "https://example.com/auth_time")
	c, err := ReadConfig(false)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/auth_time", c.AuthTimeClaim)

	t.Setenv("REEARTH_ACCOUNTS_AUTH_TIME_CLAIM", " iat ")
	_, err = ReadConfig(false)
	assert.EqualError(t, err, `REEARTH_ACCOUNTS_AUTH_TIME_CLAIM: login time claim must not be the reserved claim "iat", which does not record when the user logged in`)
}

func TestConfig_Print_MasksSecrets(t *testing.T) {
	c := Config{
		DB:               "mongodb://user:pass@localhost/db",
		RestAPIKey:       "rest-api-key-value",
		SwaggerBasicPass: "swagger-pass-value",
		SignupSecret:     "signup-secret-value",
		SyncSSOAPIKey:    "sync-sso-api-key-value",
		Auth0: Auth0Config{
			ClientSecret: "auth0-client-secret-value",
		},
	}

	got := c.Print()

	for _, secret := range []string{
		"mongodb://user:pass@localhost/db",
		"rest-api-key-value",
		"swagger-pass-value",
		"signup-secret-value",
		"sync-sso-api-key-value",
		"auth0-client-secret-value",
	} {
		assert.NotContains(t, got, secret)
	}
}

func TestCIPConfig_AuthConfig(t *testing.T) {
	// Unset project => nil (no provider added)
	assert.Nil(t, CIPConfig{}.AuthConfig())

	got := CIPConfig{ProjectID: "my-proj"}.AuthConfig()
	assert.Equal(t, &AuthConfig{
		ISS: "https://securetoken.google.com/my-proj",
		AUD: []string{"my-proj"},
	}, got)
}

func TestConfig_Auths_NoCIP_Unchanged(t *testing.T) {
	// Golden: with no CIP config, an Auth0-only Config produces exactly the
	// single Auth0 provider it produced before CIP existed.
	c := Config{
		Auth0: Auth0Config{Domain: "example.auth0.com", Audience: "aud"},
	}
	assert.Equal(t, []appx.JWTProvider{
		{ISS: "https://example.auth0.com/", AUD: []string{"aud"}},
	}, c.Auths())
}

func TestConfig_Auths_MultiIssuer_Auth0AndCIP(t *testing.T) {
	c := Config{
		Auth0: Auth0Config{Domain: "example.auth0.com", Audience: "aud"},
		CIP:   CIPConfig{ProjectID: "my-proj"},
	}
	got := c.Auths()
	assert.Contains(t, got, appx.JWTProvider{
		ISS: "https://example.auth0.com/", AUD: []string{"aud"},
	})
	assert.Contains(t, got, appx.JWTProvider{
		ISS: "https://securetoken.google.com/my-proj", AUD: []string{"my-proj"},
	})
	assert.Len(t, got, 2)
}

func TestConfig_AuthProviderAndCIPAccessors(t *testing.T) {
	c := Config{
		AuthProvider: "cip",
		CIP: CIPConfig{
			ProjectID:  "my-proj",
			TenantID:   "tenant-1",
			APIKey:     "api-key",
			AuthDomain: "my-proj.firebaseapp.com",
		},
	}
	assert.Equal(t, "cip", c.GetAuthProvider())
	assert.Equal(t, "my-proj", c.GetCIPProjectID())
	assert.Equal(t, "tenant-1", c.GetCIPTenantID())
	assert.Equal(t, "api-key", c.GetCIPAPIKey())
	assert.Equal(t, "my-proj.firebaseapp.com", c.GetCIPAuthDomain())

	// default falls back to auth0 when unset
	assert.Equal(t, "auth0", Config{}.GetAuthProvider())
}

func TestResolveDBDriver(t *testing.T) {
	assert.Equal(t, "mongo", (&Config{DB: "mongodb://localhost"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "mongodb+srv://x"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "postgres://u:p@h/db"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "postgresql://u:p@h/db"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "postgres"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "postgres://x", DBDriver: "mongo"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "postgresql"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "postgres://x", DBDriver: "mongodb"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "Postgres"}).ResolveDBDriver())
	// unknown override falls through to scheme inference
	assert.Equal(t, "postgres", (&Config{DB: "postgres://x", DBDriver: "sqlite"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "mongodb://x", DBDriver: "sqlite"}).ResolveDBDriver())
}

func TestReadConfig_UserLookupLimit(t *testing.T) {
	t.Setenv("SKIP_DOTENV", "1")

	c, err := ReadConfig(false)
	require.NoError(t, err)
	assert.False(t, c.UserLookupLimit.Enforce, "ships in log-only mode")
	assert.Positive(t, c.UserLookupLimit.PerMinute)

	t.Setenv("REEARTH_USERLOOKUPLIMIT_ENFORCE", "true")
	t.Setenv("REEARTH_USERLOOKUPLIMIT_PERMINUTE", "30")
	c, err = ReadConfig(false)
	require.NoError(t, err)
	assert.True(t, c.UserLookupLimit.Enforce)
	assert.Equal(t, 30, c.UserLookupLimit.PerMinute)
}
