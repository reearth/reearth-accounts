package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeProvider struct {
	domain, audience, webClientID string
	authProvider                  string
	cipProjectID, cipTenantID     string
	cipAPIKey, cipAuthDomain      string
}

func (f fakeProvider) GetAuth0Domain() string      { return f.domain }
func (f fakeProvider) GetAuth0Audience() string    { return f.audience }
func (f fakeProvider) GetAuth0WebClientID() string { return f.webClientID }
func (f fakeProvider) GetAuthProvider() string     { return f.authProvider }
func (f fakeProvider) GetCIPProjectID() string     { return f.cipProjectID }
func (f fakeProvider) GetCIPTenantID() string      { return f.cipTenantID }
func (f fakeProvider) GetCIPAPIKey() string        { return f.cipAPIKey }
func (f fakeProvider) GetCIPAuthDomain() string    { return f.cipAuthDomain }

func TestExtractAuthConfigData_Auth0Only(t *testing.T) {
	ac := ExtractAuthConfigData(fakeProvider{
		domain: "example.auth0.com", audience: "aud", webClientID: "cid",
		authProvider: "auth0",
	})
	assert.Equal(t, "https://example.auth0.com/", *ac.Auth0Domain)
	assert.Equal(t, "aud", *ac.Auth0Audience)
	assert.Equal(t, "cid", *ac.Auth0ClientID)
	assert.Equal(t, "auth0", *ac.AuthProvider)
	assert.Nil(t, ac.CIPProjectID)
	assert.Nil(t, ac.CIPAPIKey)
}

func TestExtractAuthConfigData_CIP(t *testing.T) {
	ac := ExtractAuthConfigData(fakeProvider{
		authProvider:  "cip",
		cipProjectID:  "my-proj",
		cipTenantID:   "tenant-1",
		cipAPIKey:     "api-key",
		cipAuthDomain: "my-proj.firebaseapp.com",
	})
	assert.Equal(t, "cip", *ac.AuthProvider)
	assert.Equal(t, "my-proj", *ac.CIPProjectID)
	assert.Equal(t, "tenant-1", *ac.CIPTenantID)
	assert.Equal(t, "api-key", *ac.CIPAPIKey)
	assert.Equal(t, "my-proj.firebaseapp.com", *ac.CIPAuthDomain)
}

func TestExtractAuthConfigData_DefaultsToAuth0(t *testing.T) {
	// An empty authProvider must resolve to "auth0" (the default), even when CIP
	// advertisement fields are present: "cip" is opt-in only.
	ac := ExtractAuthConfigData(fakeProvider{
		authProvider: "",
		cipProjectID: "my-proj",
		cipAPIKey:    "api-key",
	})
	assert.Equal(t, "auth0", *ac.AuthProvider)
	assert.Equal(t, "my-proj", *ac.CIPProjectID)
}

func TestExtractAuthConfigData_Nil(t *testing.T) {
	assert.NotNil(t, ExtractAuthConfigData(nil))
}
