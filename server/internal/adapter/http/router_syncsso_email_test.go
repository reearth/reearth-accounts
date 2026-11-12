package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncSSORoute_Email(t *testing.T) {
	ctx := context.Background()
	repos := memory.New()
	require.NoError(t, repos.Role.Save(ctx, *role.New().NewID().Name(interfaces.RoleSelf).MustBuild()))
	require.NoError(t, repos.Role.Save(ctx, *role.New().NewID().Name(role.RoleOwner.String()).MustBuild()))
	uc := &interfaces.Container{User: interactor.NewUser(repos, nil, nil, "", "")}

	other := user.New().NewID().Name("other").Email("other@example.com").Workspace(workspace.NewID()).MustBuild()
	require.NoError(t, repos.User.Save(ctx, other))

	resolver := func(c echo.Context, _ *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
		if c.Request().Header.Get("X-Test-Service") != "" {
			iid := id.NewIntegrationID()
			return nil, &workspace.Operator{Integration: &iid}, nil
		}
		uid := c.Request().Header.Get("X-Test-User")
		if uid == "" {
			return nil, nil, nil
		}
		parsed, err := id.UserIDFrom(uid)
		if err != nil {
			return nil, nil, err
		}
		u, err := repos.User.FindByID(c.Request().Context(), parsed)
		if err != nil {
			return nil, nil, err
		}
		return u, &workspace.Operator{User: &parsed}, nil
	}
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		Photos:             noSigning(t),
		AuthResolver:       resolver,
		UsecaseMiddleware: func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.SetRequest(c.Request().WithContext(adapter.AttachUsecases(c.Request().Context(), uc)))
				return next(c)
			}
		},
	})

	sync := func(header, value string) map[string]any {
		t.Helper()
		body := `{"name":"victim@example.com","email":"victim@example.com","sub":"samlp|org|victim"}`
		req := httptest.NewRequest(http.MethodPost, "/api/users/sync-sso", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set(header, value)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeJSON(t, rec)
	}

	created := sync("X-Test-Service", "1")
	assert.Equal(t, "victim@example.com", created["email"], "the service caller sees the email")

	byOther := sync("X-Test-User", other.ID().String())
	assert.Equal(t, created["id"], byOther["id"])
	assert.Equal(t, "", byOther["email"], "a signed-in user naming another user's sub must not see that email")

	bySelf := sync("X-Test-User", created["id"].(string))
	assert.Equal(t, "victim@example.com", bySelf["email"], "the synced user sees their own email")
}
