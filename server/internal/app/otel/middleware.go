package otel

import (
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var skipPaths = []string{"/"}

func openTelemetrySkipper(c echo.Context) bool {
	// Match against the actual request URL because c.Path() returns the matched
	// route pattern — unregistered paths (e.g. health checks to "/") return empty.
	if slices.Contains(skipPaths, c.Request().URL.Path) {
		return true
	}

	ua := c.Request().UserAgent()
	return strings.Contains(ua, "GoogleStackdriverMonitoring")
}

// Middleware returns an Echo middleware that adds OpenTelemetry tracing
func Middleware(serviceName string) echo.MiddlewareFunc {
	tracing := otelecho.Middleware(serviceName, otelecho.WithSkipper(openTelemetrySkipper))
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return tracing(clientAddress(next))
	}
}

// clientAddress overwrites the client.address that otelecho reads from
// X-Forwarded-For with the address Echo's IPExtractor resolves.
func clientAddress(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		trace.SpanFromContext(c.Request().Context()).SetAttributes(semconv.ClientAddress(c.RealIP()))
		return next(c)
	}
}
