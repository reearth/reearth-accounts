package gateway

import "github.com/reearth/reearthx/mailer"

// Provider is a typed identifier for an external authentication provider used
// to key per-provider authenticators on Container.
type Provider string

const (
	// ProviderAuth0 routes management calls to the Auth0 authenticator.
	ProviderAuth0 Provider = "auth0"
	// ProviderCIP routes management calls to the Cloud Identity Platform
	// (Firebase) authenticator. CIP/Firebase subs are stored unprefixed, so
	// an empty provider string on an auth record is treated as CIP.
	ProviderCIP Provider = "cip"
)

type Container struct {
	// Authenticators holds the external IdP authenticators keyed by auth provider.
	// Management calls are routed by each auth record's provider.
	Authenticators map[Provider]Authenticator
	Mailer         mailer.Mailer
	Storage        Storage
}

// AuthenticatorFor returns the authenticator for an auth record's provider, or nil
// if none is configured (callers should skip). CIP/Firebase subs are stored
// unprefixed, so their parsed provider is empty ("") and is treated as ProviderCIP.
func (c *Container) AuthenticatorFor(provider string) Authenticator {
	if c == nil {
		return nil
	}
	p := Provider(provider)
	if p == "" {
		p = ProviderCIP
	}
	return c.Authenticators[p]
}
