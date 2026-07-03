// Package session issues and verifies the admin app's own session token: a
// short-lived HS256 JWT that is stored in an HttpOnly cookie. Google's id_token
// is only used once at sign-in; the session token is what authenticates
// subsequent requests.
package session

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

var (
	// ErrInvalidToken is returned when a token is malformed, has a bad
	// signature, is expired, or carries an unusable subject.
	ErrInvalidToken = errors.New("invalid session token")
	// ErrEmptySecret is returned by Issue/Parse when no signing secret is
	// configured (NewManager itself never fails).
	ErrEmptySecret = errors.New("session secret is empty")
)

const issuer = "reearth-accounts-admin"

// CookieName is the name of the HttpOnly cookie carrying the session token.
const CookieName = "admin_session"

// Manager signs and parses session tokens using a server-side HMAC secret.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager builds a session Manager. ttl controls the token (and cookie)
// lifetime.
func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// TTL returns the configured token lifetime.
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue creates a signed session token for the given admin user, valid for TTL.
func (m *Manager) Issue(id adminuser.ID, now time.Time) (string, error) {
	if len(m.secret) == 0 {
		return "", ErrEmptySecret
	}
	claims := jwt.RegisteredClaims{
		Subject:   id.String(),
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse validates a session token and returns the admin user ID it carries.
func (m *Manager) Parse(token string) (adminuser.ID, error) {
	if len(m.secret) == 0 {
		return adminuser.ID{}, ErrEmptySecret
	}

	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer))
	if err != nil {
		return adminuser.ID{}, ErrInvalidToken
	}

	id, err := adminuser.IDFrom(claims.Subject)
	if err != nil {
		return adminuser.ID{}, ErrInvalidToken
	}
	return id, nil
}
