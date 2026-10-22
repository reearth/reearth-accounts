package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireCSRF(t *testing.T) {
	const token = "csrf-token-value"

	tests := []struct {
		name       string
		method     string
		cookie     string
		header     string
		wantCalled bool
		wantMsg    string
	}{
		{name: "GET passes without token", method: http.MethodGet, wantCalled: true},
		{name: "HEAD passes without token", method: http.MethodHead, wantCalled: true},
		{name: "OPTIONS passes without token", method: http.MethodOptions, wantCalled: true},
		{name: "POST matching token passes", method: http.MethodPost, cookie: token, header: token, wantCalled: true},
		{name: "PUT matching token passes", method: http.MethodPut, cookie: token, header: token, wantCalled: true},
		{name: "POST without header or cookie", method: http.MethodPost, wantMsg: "missing csrf token"},
		{name: "POST cookie only", method: http.MethodPost, cookie: token, wantMsg: "missing csrf token"},
		{name: "POST header only", method: http.MethodPost, header: token, wantMsg: "missing csrf token"},
		{name: "POST mismatched token", method: http.MethodPost, cookie: token, header: "other", wantMsg: "invalid csrf token"},
		{name: "PATCH mismatched token", method: http.MethodPatch, cookie: token, header: token + "x", wantMsg: "invalid csrf token"},
		{name: "DELETE without header", method: http.MethodDelete, cookie: token, wantMsg: "missing csrf token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: tt.cookie})
			}
			if tt.header != "" {
				req.Header.Set(session.CSRFHeaderName, tt.header)
			}
			c := e.NewContext(req, httptest.NewRecorder())

			called := false
			err := mw.RequireCSRF(next(&called))(c)

			assert.Equal(t, tt.wantCalled, called)
			if tt.wantCalled {
				require.NoError(t, err)
				return
			}
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, http.StatusForbidden, httpErr.Code)
			assert.Equal(t, tt.wantMsg, httpErr.Message)
		})
	}
}

func TestRequireCSRFIfCookie(t *testing.T) {
	const token = "csrf-token-value"

	tests := []struct {
		name       string
		cookie     string
		header     string
		wantCalled bool
		wantMsg    string
	}{
		{name: "no cookie passes", wantCalled: true},
		{name: "no cookie with header passes", header: token, wantCalled: true},
		{name: "matching token passes", cookie: token, header: token, wantCalled: true},
		{name: "cookie without header", cookie: token, wantMsg: "missing csrf token"},
		{name: "mismatched token", cookie: token, header: "other", wantMsg: "invalid csrf token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: tt.cookie})
			}
			if tt.header != "" {
				req.Header.Set(session.CSRFHeaderName, tt.header)
			}
			c := e.NewContext(req, httptest.NewRecorder())

			called := false
			err := mw.RequireCSRFIfCookie(next(&called))(c)

			assert.Equal(t, tt.wantCalled, called)
			if tt.wantCalled {
				require.NoError(t, err)
				return
			}
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, http.StatusForbidden, httpErr.Code)
			assert.Equal(t, tt.wantMsg, httpErr.Message)
		})
	}
}
