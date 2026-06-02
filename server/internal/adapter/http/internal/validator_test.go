package internal_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Email string `json:"email" validate:"required,email"`
}

func TestBindValidate(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"not-an-email"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	err := httpinternal.BindValidate(c, &sample{})
	assert.Error(t, err)

	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@b.com"}`))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c2 := e.NewContext(req2, httptest.NewRecorder())
	assert.NoError(t, httpinternal.BindValidate(c2, &sample{}))
}

// TestValidate_UsesJSONFieldName verifies that validation errors report the JSON
// tag name (e.g. "bar") rather than the Go struct field name (e.g. "Bar"), so
// REST error responses align with the request shape clients send.
func TestValidate_UsesJSONFieldName(t *testing.T) {
	type Foo struct {
		Bar string `json:"bar" validate:"required"`
	}

	err := httpinternal.Validate(&Foo{})
	require.Error(t, err)

	var ve validator.ValidationErrors
	require.True(t, errors.As(err, &ve))
	require.Len(t, ve, 1)
	assert.Equal(t, "bar", ve[0].Field())
	assert.NotEqual(t, "Bar", ve[0].Field())
}

// TestValidate_FallsBackToStructFieldName verifies that when no json tag is
// present the validator falls back to the Go struct field name.
func TestValidate_FallsBackToStructFieldName(t *testing.T) {
	type Foo struct {
		Bar string `validate:"required"`
	}

	err := httpinternal.Validate(&Foo{})
	require.Error(t, err)

	var ve validator.ValidationErrors
	require.True(t, errors.As(err, &ve))
	require.Len(t, ve, 1)
	assert.Equal(t, "Bar", ve[0].Field())
}
