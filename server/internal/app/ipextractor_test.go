package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mailprovider"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		proxies    string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "no list uses the connection address and ignores forwarding headers",
			remoteAddr: "198.51.100.7:4000",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "1.2.3.5"},
			want:       "198.51.100.7",
		},
		{
			name:       "no list ignores forwarding headers from loopback",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:       "127.0.0.1",
		},
		{
			name:       "trusted proxy forwards the client",
			proxies:    "10.0.0.0/8",
			remoteAddr: "10.1.2.3:4000",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.9"},
			want:       "203.0.113.9",
		},
		{
			name:       "single trusted IP",
			proxies:    "127.0.0.1",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.9"},
			want:       "203.0.113.9",
		},
		{
			name:       "chain resolves to the right-most hop that is not trusted",
			proxies:    "10.0.0.0/8, 192.0.2.10",
			remoteAddr: "10.1.2.3:4000",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.1, 203.0.113.9, 192.0.2.10"},
			want:       "203.0.113.9",
		},
		{
			name:       "untrusted peer's forwarding headers are ignored",
			proxies:    "10.0.0.0/8",
			remoteAddr: "198.51.100.7:4000",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "1.2.3.5"},
			want:       "198.51.100.7",
		},
		{
			name:       "private ranges outside the list are not trusted",
			proxies:    "10.0.0.0/8",
			remoteAddr: "10.1.2.3:4000",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.9, 192.168.1.5"},
			want:       "192.168.1.5",
		},
		{
			name:       "loopback outside the list is not trusted",
			proxies:    "10.0.0.0/8",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.9"},
			want:       "127.0.0.1",
		},
		{
			name:       "X-Real-IP from a trusted proxy is ignored",
			proxies:    "10.0.0.0/8",
			remoteAddr: "10.1.2.3:4000",
			headers:    map[string]string{"X-Real-IP": "203.0.113.9"},
			want:       "10.1.2.3",
		},
		{
			name:       "IPv6 trusted proxy",
			proxies:    "::1/128",
			remoteAddr: "[::1]:4000",
			headers:    map[string]string{"X-Forwarded-For": "2001:db8::9"},
			want:       "2001:db8::9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SKIP_DOTENV", "1")
			t.Setenv("REEARTH_ACCOUNTS_TRUSTED_PROXIES", tt.proxies)
			conf, err := ReadConfig(false)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/test/client-ip", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.want, serveClientIP(t, conf, req))
		})
	}
}

func TestReadConfig_TrustedProxies(t *testing.T) {
	t.Setenv("SKIP_DOTENV", "1")

	t.Setenv("REEARTH_ACCOUNTS_TRUSTED_PROXIES", "10.0.0.0/8,192.0.2.10,::1")
	c, err := ReadConfig(false)
	require.NoError(t, err)
	assert.Contains(t, c.Print(), "TrustedProxies:[10.0.0.0/8 192.0.2.10 ::1]")
	assert.Contains(t, c.Print(), "TrustedProxyPrefixes:[10.0.0.0/8 192.0.2.10/32 ::1/128]")

	for _, v := range []string{"10.0.0.0/33", "not-an-ip", "fe80::1%eth0"} {
		t.Setenv("REEARTH_ACCOUNTS_TRUSTED_PROXIES", v)
		_, err = ReadConfig(false)
		assert.ErrorContains(t, err, "REEARTH_ACCOUNTS_TRUSTED_PROXIES: ", v)
	}
}

func serveClientIP(t *testing.T, conf *Config, req *http.Request) string {
	t.Helper()
	conf.Mock_Auth = true
	conf.StorageIsLocal = true
	e := initEcho(context.Background(), &ServerConfig{
		Config:   conf,
		Repos:    memory.New(),
		Gateways: &gateway.Container{Mail: &mailprovider.Mock{}},
	})
	e.GET("/test/client-ip", func(c echo.Context) error {
		return c.String(http.StatusOK, c.RealIP())
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
