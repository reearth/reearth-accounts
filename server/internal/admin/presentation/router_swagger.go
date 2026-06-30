package presentation

import (
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/labstack/echo/v4"
	_ "github.com/reearth/reearth-accounts/server/docs/admin" // generated admin OpenAPI spec (make swag-admin)
)

// RegisterSwaggerRoutes serves the admin Swagger UI (non-production only).
func RegisterSwaggerRoutes(e *echo.Echo) {
	e.GET("/swagger/*", echoSwagger.WrapHandler)
}
