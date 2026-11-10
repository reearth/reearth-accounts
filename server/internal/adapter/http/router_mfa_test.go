package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMFAAuthenticator struct {
	disabledSub   string
	enrolled      bool
	recoveryCalls int
}

func (f *fakeMFAAuthenticator) DisableMFA(_ context.Context, sub string) error {
	f.disabledSub = sub
	return nil
}

func (f *fakeMFAAuthenticator) EnableMFA(_ context.Context, sub string) (string, error) {
	return "https://idp.example/enroll?sub=" + sub, nil
}

func (f *fakeMFAAuthenticator) GetMFAStatus(_ context.Context, _ string) (gateway.MFAStatus, error) {
	return gateway.MFAStatus{Enrolled: f.enrolled}, nil
}

func (f *fakeMFAAuthenticator) GetUser(_ context.Context, _ string) (gateway.AuthenticatorUser, error) {
	return gateway.AuthenticatorUser{}, nil
}

func (f *fakeMFAAuthenticator) RegenerateMFARecoveryCode(_ context.Context, _ string) (string, error) {
	f.recoveryCalls++
	return "recovery-123", nil
}

func (f *fakeMFAAuthenticator) ResendVerificationEmail(_ context.Context, _ string) error {
	return nil
}

func (f *fakeMFAAuthenticator) UpdateUser(_ context.Context, _ gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	return gateway.AuthenticatorUser{}, nil
}

type mfaFixture struct {
	auth      *fakeMFAAuthenticator
	e         *echo.Echo
	plainUser *user.User
	auth0User *user.User
}

func newMFAFixture(t *testing.T) *mfaFixture {
	t.Helper()
	return newMFAFixtureWithMaxAuthAge(t, 0)
}

// The fake JWT middleware stands in for appx: it attaches a token whose
// auth_time is the X-Test-Auth-Time header, when the header is present.
func fakeJWTMiddleware(t *testing.T) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			v := c.Request().Header.Get("X-Test-Auth-Time")
			if v == "" {
				return next(c)
			}
			authTime, err := strconv.ParseInt(v, 10, 64)
			require.NoError(t, err)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"auth_time": authTime}).SignedString([]byte("test-key"))
			require.NoError(t, err)
			ctx := context.WithValue(c.Request().Context(), adapter.AuthInfoKey, appx.AuthInfo{Token: token})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func newMFAFixtureWithMaxAuthAge(t *testing.T, maxAuthAge time.Duration) *mfaFixture {
	t.Helper()
	ctx := context.Background()
	repos := memory.New()

	auth0User := user.New().NewID().Name("auth0 user").Email("a@example.com").
		Workspace(workspace.NewID()).
		Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|alice"}}).
		MustBuild()
	plainUser := user.New().NewID().Name("plain user").Email("p@example.com").
		Workspace(workspace.NewID()).
		MustBuild()
	require.NoError(t, repos.User.Save(ctx, auth0User))
	require.NoError(t, repos.User.Save(ctx, plainUser))

	fake := &fakeMFAAuthenticator{enrolled: true}
	gw := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderAuth0: fake}}
	uc := interactor.NewContainer(repos, gw, nil, nil, interactor.ContainerConfig{MFARecoveryMaxAuthAge: maxAuthAge})

	users := map[string]*user.User{
		auth0User.ID().String(): auth0User,
		plainUser.ID().String(): plainUser,
	}
	resolver := func(c echo.Context, _ *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
		u := users[c.Request().Header.Get("X-Test-User")]
		if u == nil {
			return nil, nil, nil
		}
		uid := u.ID()
		return u, &workspace.Operator{User: &uid}, nil
	}
	usecases := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(adapter.AttachUsecases(c.Request().Context(), &uc)))
			return next(c)
		}
	}

	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		AuthResolver:       resolver,
		CacheControl:       cacheControlMiddleware,
		JWTMiddleware:      fakeJWTMiddleware(t),
		UsecaseMiddleware:  usecases,
	})
	return &mfaFixture{auth: fake, e: e, plainUser: plainUser, auth0User: auth0User}
}

func (f *mfaFixture) do(method, path string, u *user.User) *httptest.ResponseRecorder {
	return f.doAt(method, path, u, nil)
}

func (f *mfaFixture) doAt(method, path string, u *user.User, authTime *time.Time) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if u != nil {
		req.Header.Set("X-Test-User", u.ID().String())
	}
	if authTime != nil {
		req.Header.Set("X-Test-Auth-Time", strconv.FormatInt(authTime.Unix(), 10))
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestMFARoutes(t *testing.T) {
	routes := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/users/me/mfa"},
		{http.MethodPost, "/api/users/me/mfa"},
		{http.MethodDelete, "/api/users/me/mfa"},
		{http.MethodPost, "/api/users/me/mfa/recovery-code"},
	}

	t.Run("status", func(t *testing.T) {
		f := newMFAFixture(t)
		rec := f.do(http.MethodGet, "/api/users/me/mfa", f.auth0User)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]any{"enrolled": true}, decodeJSON(t, rec))
	})

	t.Run("status without auth0 identity is not enrolled", func(t *testing.T) {
		f := newMFAFixture(t)
		rec := f.do(http.MethodGet, "/api/users/me/mfa", f.plainUser)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]any{"enrolled": false}, decodeJSON(t, rec))
	})

	t.Run("enable returns the enrollment url for the caller", func(t *testing.T) {
		f := newMFAFixture(t)
		rec := f.do(http.MethodPost, "/api/users/me/mfa", f.auth0User)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]any{"enrollment_url": "https://idp.example/enroll?sub=auth0|alice"}, decodeJSON(t, rec))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	})

	t.Run("disable acts on the caller", func(t *testing.T) {
		f := newMFAFixture(t)
		rec := f.do(http.MethodDelete, "/api/users/me/mfa", f.auth0User)
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "auth0|alice", f.auth.disabledSub)
	})

	t.Run("recovery code is not cacheable", func(t *testing.T) {
		f := newMFAFixture(t)
		rec := f.do(http.MethodPost, "/api/users/me/mfa/recovery-code", f.auth0User)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]any{"recovery_code": "recovery-123"}, decodeJSON(t, rec))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	})

	t.Run("unauthenticated is 401", func(t *testing.T) {
		f := newMFAFixture(t)
		for _, r := range routes {
			rec := f.do(r.method, r.path, nil)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s", r.method, r.path)
		}
	})

	t.Run("mutations without auth0 identity are 400", func(t *testing.T) {
		f := newMFAFixture(t)
		for _, r := range routes[1:] {
			rec := f.do(r.method, r.path, f.plainUser)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "%s %s", r.method, r.path)
			assert.Equal(t, "no authenticator found", decodeJSON(t, rec)["description"], "%s %s", r.method, r.path)
		}
		assert.Empty(t, f.auth.disabledSub)
	})

	t.Run("recovery code needs a recent login when the window is on", func(t *testing.T) {
		f := newMFAFixtureWithMaxAuthAge(t, 5*time.Minute)
		old := time.Now().Add(-time.Hour)
		recent := time.Now().Add(-time.Minute)
		const path = "/api/users/me/mfa/recovery-code"

		for name, authTime := range map[string]*time.Time{"no auth_time": nil, "old login": &old} {
			rec := f.doAt(http.MethodPost, path, f.auth0User, authTime)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
			assert.Equal(t, `Bearer error="insufficient_user_authentication"`, rec.Header().Get("WWW-Authenticate"), name)
			body := decodeJSON(t, rec)
			assert.Equal(t, "recent_login_required", body["message"], name)
			assert.Equal(t, "recent login required", body["description"], name)
		}
		assert.Zero(t, f.auth.recoveryCalls)

		rec := f.doAt(http.MethodPost, path, f.auth0User, &recent)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]any{"recovery_code": "recovery-123"}, decodeJSON(t, rec))
	})
}
