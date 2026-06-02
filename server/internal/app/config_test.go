package app

import (
	"testing"

	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
)

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
