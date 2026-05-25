package gateway

import "github.com/reearth/reearthx/mailer"

type Container struct {
	// Authenticators holds the external IdP authenticators keyed by auth provider
	// ("auth0", "cip"). Management calls are routed by each auth record's provider.
	Authenticators map[string]Authenticator
	Mailer         mailer.Mailer
	Storage        Storage
}

// AuthenticatorFor returns the authenticator for an auth record's provider, or nil
// if none is configured (callers should skip). CIP/Firebase subs are stored
// unprefixed, so their parsed provider is empty ("") and is treated as "cip".
func (c *Container) AuthenticatorFor(provider string) Authenticator {
	if c == nil {
		return nil
	}
	if provider == "" {
		provider = "cip"
	}
	return c.Authenticators[provider]
}
