package middleware_test

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func next(called *bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		*called = true
		return c.NoContent(http.StatusOK)
	}
}
