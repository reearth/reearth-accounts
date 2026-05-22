package adapter

import (
	"strings"

	"github.com/samber/lo"
)

// AuthConfigData represents public auth configuration that can be exposed to clients.
type AuthConfigData struct {
	Auth0Domain   *string
	Auth0Audience *string
	Auth0ClientID *string
	AuthProvider  *string
	// CIP (Cloud Identity Platform) public client values.
	CIPAPIKey     *string
	CIPAuthDomain *string
	CIPProjectID  *string
	CIPTenantID   *string
}

// AuthConfigProvider is implemented by app.Config; it avoids an import cycle.
type AuthConfigProvider interface {
	GetAuth0Domain() string
	GetAuth0Audience() string
	GetAuth0WebClientID() string
	GetAuthProvider() string
	GetCIPAPIKey() string
	GetCIPAuthDomain() string
	GetCIPProjectID() string
	GetCIPTenantID() string
}

// Auth0ConfigProvider is retained as an alias for backward compatibility.
type Auth0ConfigProvider = AuthConfigProvider

// ExtractAuthConfigData extracts public auth configuration from a provider.
func ExtractAuthConfigData(provider AuthConfigProvider) *AuthConfigData {
	if provider == nil {
		return &AuthConfigData{}
	}

	ac := &AuthConfigData{}

	if domain := provider.GetAuth0Domain(); domain != "" {
		if !strings.HasPrefix(domain, "https://") && !strings.HasPrefix(domain, "http://") {
			domain = "https://" + domain
		}
		if !strings.HasSuffix(domain, "/") {
			domain = domain + "/"
		}
		ac.Auth0Domain = lo.ToPtr(domain)
	}
	if audience := provider.GetAuth0Audience(); audience != "" {
		ac.Auth0Audience = lo.ToPtr(audience)
	}
	if clientID := provider.GetAuth0WebClientID(); clientID != "" {
		ac.Auth0ClientID = lo.ToPtr(clientID)
	}

	if v := provider.GetCIPAPIKey(); v != "" {
		ac.CIPAPIKey = lo.ToPtr(v)
	}
	if v := provider.GetCIPAuthDomain(); v != "" {
		ac.CIPAuthDomain = lo.ToPtr(v)
	}
	if v := provider.GetCIPProjectID(); v != "" {
		ac.CIPProjectID = lo.ToPtr(v)
	}
	if v := provider.GetCIPTenantID(); v != "" {
		ac.CIPTenantID = lo.ToPtr(v)
	}

	// AuthProvider: honor explicit config; otherwise infer from what is set.
	switch provider.GetAuthProvider() {
	case "cip":
		ac.AuthProvider = lo.ToPtr("cip")
	case "auth0":
		ac.AuthProvider = lo.ToPtr("auth0")
	default:
		if ac.CIPProjectID != nil {
			ac.AuthProvider = lo.ToPtr("cip")
		} else if ac.Auth0Domain != nil {
			ac.AuthProvider = lo.ToPtr("auth0")
		}
	}

	return ac
}
