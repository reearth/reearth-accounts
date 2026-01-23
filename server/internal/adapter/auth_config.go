package adapter

import (
	"strings"

	"github.com/samber/lo"
)

// AuthConfigData represents public auth configuration that can be exposed to clients
type AuthConfigData struct {
	Auth0Domain    *string
	Auth0Audience  *string
	Auth0ClientID  *string
	AuthProvider   *string
}

// Auth0ConfigProvider is an interface to avoid import cycle with app.Config
type Auth0ConfigProvider interface {
	GetAuth0Domain() string
	GetAuth0Audience() string
	GetAuth0WebClientID() string
}

// ExtractAuthConfigData extracts public auth configuration from a provider
func ExtractAuthConfigData(provider Auth0ConfigProvider) *AuthConfigData {
	if provider == nil {
		return &AuthConfigData{}
	}

	ac := &AuthConfigData{}

	// Get Auth0 configuration if available
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

	// Set default auth provider
	if ac.Auth0Domain != nil {
		ac.AuthProvider = lo.ToPtr("auth0")
	}

	return ac
}
