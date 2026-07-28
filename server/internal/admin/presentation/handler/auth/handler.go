// Package auth implements the admin authentication endpoints: Google sign-in,
// logout, and the current-user lookup, backed by an HttpOnly session cookie
// plus a readable (non-HttpOnly) admin_csrf double-submit companion cookie.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
)

// csrfCookieName is the name of the non-HttpOnly companion cookie carrying the
// double-submit CSRF token. The frontend JS reads it via document.cookie and
// echoes it back in a request header; the admin BFF matches the two.
const csrfCookieName = "admin_csrf"

// CookieSecure is a named bool so Wire can inject it unambiguously. It controls
// the Secure attribute of the session cookie (true in production/HTTPS).
type CookieSecure bool

// Handler serves the admin auth endpoints.
type Handler struct {
	signIn *authuc.GoogleSignInUseCase
	getMe  *authuc.GetMeUseCase
	sess   *session.Manager
	secure bool
}

// NewHandler is a Wire provider for the auth Handler.
func NewHandler(signIn *authuc.GoogleSignInUseCase, getMe *authuc.GetMeUseCase, sess *session.Manager, secure CookieSecure) *Handler {
	return &Handler{signIn: signIn, getMe: getMe, sess: sess, secure: bool(secure)}
}

func (h *Handler) newSessionCookie(value string, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     session.CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(h.sess.TTL()),
		MaxAge:   int(h.sess.TTL().Seconds()),
	}
}

func (h *Handler) clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     session.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		// Also set a past Expires for clients/proxies that honor the legacy
		// RFC6265 deletion behavior rather than Max-Age.
		Expires: time.Unix(0, 0),
	}
}

// newCSRFCookie builds the double-submit CSRF cookie. It mirrors
// newSessionCookie but is deliberately NOT HttpOnly so the frontend JS can read
// its value and echo it back in a request header.
func (h *Handler) newCSRFCookie(value string, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(h.sess.TTL()),
		MaxAge:   int(h.sess.TTL().Seconds()),
	}
}

func (h *Handler) clearCSRFCookie() *http.Cookie {
	return &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		// Also set a past Expires for clients/proxies that honor the legacy
		// RFC6265 deletion behavior rather than Max-Age.
		Expires: time.Unix(0, 0),
	}
}

// newCSRFToken returns an unguessable, URL-safe token suitable as both a cookie
// value and an HTTP header value. Double-submit validation only requires that
// the cookie and header values match, so the token needs no server-side storage
// and is independent of the session JWT.
func newCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
