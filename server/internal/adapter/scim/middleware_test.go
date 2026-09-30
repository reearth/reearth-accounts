package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestScimBearerAuth_MissingHeader(t *testing.T) {
	db := accountmemory.New()
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := ScimBearerAuth(db.Workspace)
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScimBearerAuth_InvalidToken(t *testing.T) {
	db := accountmemory.New()

	// Create workspace with SCIM enabled and a known token hash.
	ownerID := user.NewID()
	plaintext := "valid-token-abc"
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	require.NoError(t, err)

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	cfg.SetTokenHash(string(hash))

	ws := workspace.New().
		NewID().
		Name("test").
		Alias("test").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), ws))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := ScimBearerAuth(db.Workspace)
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err = handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScimBearerAuth_ValidToken(t *testing.T) {
	db := accountmemory.New()

	ownerID := user.NewID()
	plaintext := "valid-token-abc"
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	require.NoError(t, err)

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	cfg.SetTokenHash(string(hash))

	ws := workspace.New().
		NewID().
		Name("test").
		Alias("test").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), ws))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedWSID workspace.ID
	mw := ScimBearerAuth(db.Workspace)
	handler := mw(func(c echo.Context) error {
		id, ok := WorkspaceIDFromContext(c.Request().Context())
		require.True(t, ok, "workspace ID must be in context")
		capturedWSID = id
		return c.String(http.StatusOK, "ok")
	})

	err = handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ws.ID(), capturedWSID)
}

func TestScimBearerAuth_DisabledWorkspace(t *testing.T) {
	db := accountmemory.New()

	ownerID := user.NewID()
	plaintext := "valid-token-abc"
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	require.NoError(t, err)

	// SCIM not enabled
	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(false)
	cfg.SetTokenHash(string(hash))

	ws := workspace.New().
		NewID().
		Name("test").
		Alias("test").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), ws))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := ScimBearerAuth(db.Workspace)
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err = handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
