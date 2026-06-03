package sso

import "slices"

type ConnectionType string

const (
	ConnectionTypeAzureAD         ConnectionType = "azure_ad"
	ConnectionTypeGoogleWorkspace ConnectionType = "google_workspace"
	ConnectionTypeOIDC            ConnectionType = "oidc"
	ConnectionTypeOkta            ConnectionType = "okta"
	ConnectionTypeSAML            ConnectionType = "saml"
)

type Config struct {
	auth0ConnectionID string
	auth0OrgID        string
	clientID          string
	clientSecret      string
	connectionType    ConnectionType
	directoryDomain   string
	emailDomains      []string
	enabled           bool
	jitDefaultRole    string
	oidcClientID      string
	oidcClientSecret  string
	oidcDiscoveryURL  string
	samlEntityID      string
	samlMetadataURL   string
	samlSignInURL     string
	samlSignOutURL    string
	samlX509Cert      string
	verified          bool
}

func New(connectionType ConnectionType) *Config {
	return &Config{connectionType: connectionType}
}

func (s *Config) Auth0ConnectionID() string    { return s.auth0ConnectionID }
func (s *Config) Auth0OrgID() string           { return s.auth0OrgID }
func (s *Config) ClientID() string             { return s.clientID }
func (s *Config) ClientSecret() string         { return s.clientSecret }
func (s *Config) ConnectionType() ConnectionType { return s.connectionType }
func (s *Config) DirectoryDomain() string      { return s.directoryDomain }
func (s *Config) EmailDomains() []string       { return append([]string(nil), s.emailDomains...) }
func (s *Config) Enabled() bool                { return s.enabled }
func (s *Config) HasEmailDomain(domain string) bool {
	return slices.Contains(s.emailDomains, domain)
}
func (s *Config) JITDefaultRole() string   { return s.jitDefaultRole }
func (s *Config) OIDCClientID() string     { return s.oidcClientID }
func (s *Config) OIDCClientSecret() string { return s.oidcClientSecret }
func (s *Config) OIDCDiscoveryURL() string { return s.oidcDiscoveryURL }
func (s *Config) SAMLEntityID() string     { return s.samlEntityID }
func (s *Config) SAMLMetadataURL() string  { return s.samlMetadataURL }
func (s *Config) SAMLSignInURL() string    { return s.samlSignInURL }
func (s *Config) SAMLSignOutURL() string   { return s.samlSignOutURL }
func (s *Config) SAMLX509Cert() string     { return s.samlX509Cert }
func (s *Config) Verified() bool           { return s.verified }

func (s *Config) SetAuth0ConnectionID(v string)      { s.auth0ConnectionID = v }
func (s *Config) SetAuth0OrgID(v string)             { s.auth0OrgID = v }
func (s *Config) SetClientID(v string)               { s.clientID = v }
func (s *Config) SetClientSecret(v string)           { s.clientSecret = v }
func (s *Config) SetConnectionType(v ConnectionType) { s.connectionType = v }
func (s *Config) SetDirectoryDomain(v string)        { s.directoryDomain = v }
func (s *Config) SetEmailDomains(v []string)         { s.emailDomains = append([]string(nil), v...) }
func (s *Config) SetEnabled(v bool)                  { s.enabled = v }
func (s *Config) SetJITDefaultRole(v string)         { s.jitDefaultRole = v }
func (s *Config) SetOIDCClientID(v string)           { s.oidcClientID = v }
func (s *Config) SetOIDCClientSecret(v string)       { s.oidcClientSecret = v }
func (s *Config) SetOIDCDiscoveryURL(v string)       { s.oidcDiscoveryURL = v }
func (s *Config) SetSAMLEntityID(v string)           { s.samlEntityID = v }
func (s *Config) SetSAMLMetadataURL(v string)        { s.samlMetadataURL = v }
func (s *Config) SetSAMLSignInURL(v string)          { s.samlSignInURL = v }
func (s *Config) SetSAMLSignOutURL(v string)         { s.samlSignOutURL = v }
func (s *Config) SetSAMLX509Cert(v string)           { s.samlX509Cert = v }
func (s *Config) SetVerified(v bool)                 { s.verified = v }
