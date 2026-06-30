package di

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// echoValidator adapts go-playground/validator to echo.Validator so handlers
// can call c.Validate(&req) after c.Bind.
type echoValidator struct {
	validate *validator.Validate
}

func newValidator() *echoValidator {
	return &echoValidator{validate: validator.New()}
}

func (v *echoValidator) Validate(i any) error {
	if err := v.validate.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}
