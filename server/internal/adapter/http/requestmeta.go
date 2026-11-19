package http

import (
	"net/netip"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

// AttachClient records the calling client for policy acceptances. The IP
// comes from echo's IPExtractor, which reads X-Forwarded-For only from
// trusted proxies.
func AttachClient(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		c.SetRequest(req.WithContext(adapter.AttachClient(req.Context(), ClientOf(c))))
		return next(c)
	}
}

func ClientOf(c echo.Context) legal.Client {
	req := c.Request()
	ip, _ := netip.ParseAddr(c.RealIP())
	return legal.Client{Origin: req.Header.Get("Origin"), IP: ip.Unmap(), UserAgent: req.UserAgent()}
}
