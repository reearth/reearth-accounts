package internal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/stretchr/testify/assert"
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
