package sso

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cfg := New(ConnectionTypeSAML)
	assert.Equal(t, ConnectionTypeSAML, cfg.ConnectionType())
	assert.False(t, cfg.Enabled())
	assert.Empty(t, cfg.EmailDomains())
	assert.False(t, cfg.Verified())
}

func TestConfig_HasEmailDomain(t *testing.T) {
	cfg := New(ConnectionTypeOIDC)
	cfg.SetEmailDomains([]string{"example.com", "corp.org"})

	assert.True(t, cfg.HasEmailDomain("example.com"))
	assert.True(t, cfg.HasEmailDomain("corp.org"))
	assert.False(t, cfg.HasEmailDomain("other.com"))
	assert.False(t, cfg.HasEmailDomain(""))
}

func TestConfig_EmailDomainsIsolation(t *testing.T) {
	cfg := New(ConnectionTypeSAML)
	original := []string{"a.com", "b.com"}
	cfg.SetEmailDomains(original)

	original[0] = "changed.com"
	assert.Equal(t, "a.com", cfg.EmailDomains()[0])

	returned := cfg.EmailDomains()
	returned[0] = "changed.com"
	assert.Equal(t, "a.com", cfg.EmailDomains()[0])
}

func TestConfig_SAMLFields(t *testing.T) {
	cfg := New(ConnectionTypeSAML)
	cfg.SetSAMLSignInURL("https://idp.example.com/sso")
	cfg.SetSAMLSignOutURL("https://idp.example.com/slo")
	cfg.SetSAMLX509Cert("-----BEGIN CERTIFICATE-----\nMIIC...")
	cfg.SetSAMLMetadataURL("https://idp.example.com/metadata")
	cfg.SetSAMLEntityID("urn:example:sp")

	assert.Equal(t, "https://idp.example.com/sso", cfg.SAMLSignInURL())
	assert.Equal(t, "https://idp.example.com/slo", cfg.SAMLSignOutURL())
	assert.Contains(t, cfg.SAMLX509Cert(), "CERTIFICATE")
	assert.Equal(t, "https://idp.example.com/metadata", cfg.SAMLMetadataURL())
	assert.Equal(t, "urn:example:sp", cfg.SAMLEntityID())
}

func TestConfig_OIDCFields(t *testing.T) {
	cfg := New(ConnectionTypeOIDC)
	cfg.SetOIDCDiscoveryURL("https://accounts.google.com/.well-known/openid-configuration")
	cfg.SetOIDCClientID("client-id-123")
	cfg.SetOIDCClientSecret("super-secret")

	assert.Equal(t, "https://accounts.google.com/.well-known/openid-configuration", cfg.OIDCDiscoveryURL())
	assert.Equal(t, "client-id-123", cfg.OIDCClientID())
	assert.Equal(t, "super-secret", cfg.OIDCClientSecret())
}

func TestConfig_Auth0Fields(t *testing.T) {
	cfg := New(ConnectionTypeAzureAD)
	cfg.SetAuth0ConnectionID("con_abc123")
	cfg.SetAuth0OrgID("org_xyz789")
	cfg.SetVerified(true)

	assert.Equal(t, "con_abc123", cfg.Auth0ConnectionID())
	assert.Equal(t, "org_xyz789", cfg.Auth0OrgID())
	assert.True(t, cfg.Verified())
}
