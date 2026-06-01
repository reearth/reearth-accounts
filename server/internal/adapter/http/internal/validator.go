package internal

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/labstack/echo/v4"
)

var (
	validate   = validator.New(validator.WithRequiredStructEnabled())
	translator ut.Translator
)

func init() {
	enLocale := en.New()
	uni := ut.New(enLocale, enLocale)
	translator, _ = uni.GetTranslator("en")
	_ = enTranslations.RegisterDefaultTranslations(validate, translator)
	// Report the JSON field name (e.g. "email") in validation errors instead of the
	// Go struct field name (e.g. "Email") so REST error responses match the request
	// shape clients send and expect.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return fld.Name
		}
		return name
	})
}

func translateFieldError(fe validator.FieldError) string {
	if translator != nil {
		return fe.Translate(translator)
	}
	return fe.Error()
}

// BindValidate binds the request body/query/path into v then runs struct validation.
// Errors are surfaced through CustomHTTPErrorHandler as structured ErrorResponse JSON.
func BindValidate(c echo.Context, v any) error {
	if err := c.Bind(v); err != nil {
		return &ErrorResponse{Status: http.StatusBadRequest, Message: "invalid request", Description: "request body or parameters could not be parsed", Err: err}
	}
	if err := validate.Struct(v); err != nil {
		return err // validator.ValidationErrors handled by mapError
	}
	return nil
}

// Validate runs struct validation without binding (for already-populated structs).
func Validate(v any) error { return validate.Struct(v) }
