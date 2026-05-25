package internal_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func handleStatus(t *testing.T, err error) int {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	httpinternal.CustomHTTPErrorHandler(err, c)
	return rec.Code
}

func TestCustomHTTPErrorHandler(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, handleStatus(t, rerror.ErrNotFound))
	assert.Equal(t, http.StatusConflict, handleStatus(t, interfaces.ErrUserAlreadyExists))
	assert.Equal(t, http.StatusConflict, handleStatus(t, interfaces.ErrUserAliasAlreadyExists))
	assert.Equal(t, http.StatusForbidden, handleStatus(t, interfaces.ErrPermissionDenied))
	assert.Equal(t, http.StatusForbidden, handleStatus(t, interfaces.ErrOperationDenied))
	assert.Equal(t, http.StatusUnauthorized, handleStatus(t, httpinternal.ErrUnauthorized))
	assert.Equal(t, http.StatusUnauthorized, handleStatus(t, interfaces.ErrInvalidOperator))
	assert.Equal(t, http.StatusBadRequest, handleStatus(t, interfaces.ErrInvalidPhotoURL))
	assert.Equal(t, http.StatusInternalServerError, handleStatus(t, assert.AnError))
}
