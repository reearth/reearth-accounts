// Package auth implements the admin authentication endpoints: Google sign-in,
// logout, and the current-user lookup, backed by an HttpOnly session cookie.
package auth

import (
	"net/http"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
)

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
	}
}
