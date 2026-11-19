package di

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppEchoServer_ClientIP(t *testing.T) {
	tests := []struct {
		name       string
		proxies    string
		remoteAddr string
		want       string
	}{
		{name: "no list ignores forwarding headers", remoteAddr: "198.51.100.7:4000", want: "198.51.100.7"},
		{name: "untrusted peer's forwarding headers are ignored", proxies: "10.0.0.0/8", remoteAddr: "198.51.100.7:4000", want: "198.51.100.7"},
		{name: "trusted proxy forwards the client", proxies: "10.0.0.0/8", remoteAddr: "10.1.2.3:4000", want: "1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SKIP_DOTENV", "1")
			t.Setenv("REEARTH_ACCOUNTS_TRUSTED_PROXIES", tt.proxies)
			s := NewAppEchoServer(LoadConfig(), &presentation.Handler{}, presentation.NewAppMiddlewares())
			s.echo.GET("/test/client-ip", func(c echo.Context) error {
				return c.String(http.StatusOK, c.RealIP())
			})

			req := httptest.NewRequest(http.MethodGet, "/test/client-ip", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", "1.2.3.4")
			req.Header.Set("X-Real-IP", "1.2.3.5")
			rec := httptest.NewRecorder()
			s.echo.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.want, rec.Body.String())
		})
	}
}
