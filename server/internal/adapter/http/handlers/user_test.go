package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/handlers"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserHandlerTest(t *testing.T) *interfaces.Container {
	t.Helper()
	ctx := context.Background()
	db := accountmemory.New()

	selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
	ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
	require.NoError(t, db.Role.Save(ctx, *selfRole))
	require.NoError(t, db.Role.Save(ctx, *ownerRole))

	return &interfaces.Container{User: interactor.NewUser(db, nil, nil, "", "")}
}

func newSyncSSOEchoCtx(t *testing.T, e *echo.Echo, body string, uc *interfaces.Container) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/users/sync-sso", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adapter.AttachUsecases(req.Context(), uc))
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// TestUserHandler_SyncSSOUser_NamePrefix verifies that the provisioned user's name
// starts with "user-", preserving the intake-form trigger on first login.
func TestUserHandler_SyncSSOUser_NamePrefix(t *testing.T) {
	uc := setupUserHandlerTest(t)
	h := handlers.NewUserHandler()
	e := echo.New()

	body := `{"name":"alice@example.com","email":"alice@example.com","sub":"samlp|org|alice"}`
	c, rec := newSyncSSOEchoCtx(t, e, body, uc)

	require.NoError(t, h.SyncSSOUser(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp httpmodel.UserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, strings.HasPrefix(resp.Name, "user-"),
		"name must start with 'user-' to trigger the intake form on first login, got %q", resp.Name)
}

// TestUserHandler_SyncSSOUser_UniqueNames verifies that two different SSO users
// provisioned in sequence receive distinct placeholder names, preventing the
// workspace alias collision that occurred when the name was always "user-".
func TestUserHandler_SyncSSOUser_UniqueNames(t *testing.T) {
	uc := setupUserHandlerTest(t)
	h := handlers.NewUserHandler()
	e := echo.New()

	provision := func(email, sub string) httpmodel.UserResponse {
		t.Helper()
		body := `{"name":"` + email + `","email":"` + email + `","sub":"` + sub + `"}`
		c, rec := newSyncSSOEchoCtx(t, e, body, uc)
		require.NoError(t, h.SyncSSOUser(c))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp httpmodel.UserResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	// Use distinct IdP provider prefixes: in-memory ContainAuth matches on provider
	// alone, so two "samlp|..." subs would collapse to one user.
	alice := provision("alice@example.com", "samlp|org|alice")
	bob := provision("bob@example.com", "oidc|org|bob")

	assert.True(t, strings.HasPrefix(alice.Name, "user-"))
	assert.True(t, strings.HasPrefix(bob.Name, "user-"))
	assert.NotEqual(t, alice.Name, bob.Name,
		"each provisioned user must get a unique placeholder name to avoid workspace alias collisions")
}
