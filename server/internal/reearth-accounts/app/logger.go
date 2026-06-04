package app

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearthx/log"
)

func AccessLogger(l *log.Echo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			internalService := req.Header.Get("X-Internal-Service")

			if internalService != "" {
				logger := log.GetLoggerFromContext(c.Request().Context())
				if logger == nil {
					logger = log.New()
				}
				logger = logger.WithCaller(false)
				// outcoming log
				logger.Infof(
					"request from: %s",
					internalService,
				)
			}

			return l.AccessLogger()(next)(c)
		}
	}
}
