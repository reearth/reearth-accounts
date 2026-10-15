package scim

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newScimWorkspace(t *testing.T, db *repo.Container, alias string, enabled bool) (*workspace.Workspace, string) {
	t.Helper()
	ownerID := user.NewID()
	ws := workspace.New().
		NewID().
		Name("test").
		Alias(alias).
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		MustBuild()

	plaintext := ws.ID().String() + ".secretpart"
	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(enabled)
	cfg.SetTokenHash(sha256hex(plaintext))
	ws.SetScimConfig(cfg)
	require.NoError(t, db.Workspace.Save(t.Context(), ws))
	return ws, plaintext
}

func TestScimBearerAuth_MissingHeader(t *testing.T) {
	db := accountmemory.New()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := ScimBearerAuth(db.Workspace)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScimBearerAuth_ValidToken(t *testing.T) {
	db := accountmemory.New()
	ws, plaintext := newScimWorkspace(t, db, "ws-valid", true)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedWSID workspace.ID
	err := ScimBearerAuth(db.Workspace)(func(c echo.Context) error {
		id, ok := WorkspaceIDFromContext(c.Request().Context())
		require.True(t, ok)
		capturedWSID = id
		return c.String(http.StatusOK, "ok")
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ws.ID(), capturedWSID)
}

func TestScimBearerAuth_WrongSecret(t *testing.T) {
	db := accountmemory.New()
	ws, _ := newScimWorkspace(t, db, "ws-wrong", true)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer "+ws.ID().String()+".wrongsecret")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := ScimBearerAuth(db.Workspace)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScimBearerAuth_MalformedToken(t *testing.T) {
	db := accountmemory.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer no-workspace-prefix")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := ScimBearerAuth(db.Workspace)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestScimBearerAuth_DisabledWorkspace(t *testing.T) {
	db := accountmemory.New()
	_, plaintext := newScimWorkspace(t, db, "ws-disabled", false)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := ScimBearerAuth(db.Workspace)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}