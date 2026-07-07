// Package google verifies Google Identity Services id_tokens for the admin app.
package google

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"
)

// ErrInvalidIDToken is returned when the id_token fails signature/audience/
// expiry validation.
var ErrInvalidIDToken = errors.New("invalid google id_token")

// Claims holds the subset of Google id_token claims the admin auth flow needs.
type Claims struct {
	Email         string
	EmailVerified bool
	HD            string // Google Workspace hosted domain
	Name          string
	PictureURL    string
}

// Verifier validates a Google id_token and extracts its claims. It is an
// interface so the usecase layer can be tested with a fake.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (*Claims, error)
}

type verifier struct {
	clientID string
}

// NewVerifier returns a Verifier that validates tokens against the given OAuth
// client ID (the token's audience). The underlying idtoken package fetches and
// caches Google's JWKS internally.
func NewVerifier(clientID string) Verifier {
	return &verifier{clientID: clientID}
}

func (v *verifier) Verify(ctx context.Context, idToken string) (*Claims, error) {
	payload, err := idtoken.Validate(ctx, idToken, v.clientID)
	if err != nil {
		return nil, ErrInvalidIDToken
	}

	return &Claims{
		Email:         stringClaim(payload.Claims, "email"),
		EmailVerified: boolClaim(payload.Claims, "email_verified"),
		HD:            stringClaim(payload.Claims, "hd"),
		Name:          stringClaim(payload.Claims, "name"),
		PictureURL:    stringClaim(payload.Claims, "picture"),
	}, nil
}

func stringClaim(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolClaim(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
