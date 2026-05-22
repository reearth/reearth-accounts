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
