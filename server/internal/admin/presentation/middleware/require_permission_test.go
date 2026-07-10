package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newContext(t *testing.T) (echo.Context, *bool) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	return c, &called
}

func next(called *bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		*called = true
		return c.NoContent(http.StatusOK)
	}
}

func TestRequirePermission_NoAdminUser_Unauthorized(t *testing.T) {
	c, called := newContext(t)

	m := mw.RequirePermission(authz.NewChecker(nil), adminrbac.ResourceUser, adminrbac.ActionList)
	err := m(next(called))(c)

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	assert.False(t, *called)
}

func TestRequirePermission_NilClient_Bypass(t *testing.T) {
	c, called := newContext(t)
	u := adminuser.New().NewID().
		Email("op@eukarya.io").Name("op").
		Status(adminuser.StatusApproved).Role(adminuser.RoleViewer).MustBuild()
	internal.SetAdminUser(c, u)

	m := mw.RequirePermission(authz.NewChecker(nil), adminrbac.ResourceUser, adminrbac.ActionList)
	err := m(next(called))(c)

	require.NoError(t, err)
	assert.True(t, *called)
	assert.Equal(t, http.StatusOK, c.Response().Status)
}

func TestRequirePermission_InvalidRole_Forbidden(t *testing.T) {
	c, called := newContext(t)
	// A non-nil (but never-dialed) client forces the checker past the nil-client
	// bypass; an empty role is then denied before any Cerbos call is made.
	client, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
	require.NoError(t, err)

	u := adminuser.New().NewID().
		Email("op@eukarya.io").Name("op").
		Status(adminuser.StatusApproved).MustBuild()
	internal.SetAdminUser(c, u)

	m := mw.RequirePermission(authz.NewChecker(client), adminrbac.ResourceUser, adminrbac.ActionList)
	err = m(next(called))(c)

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusForbidden, httpErr.Code)
	assert.False(t, *called)
}
