package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func() context.Context
		setupConfig    func() *ServerConfig
		setupRequest   func() *http.Request
		expectedStatus int
		assertContext  func(t *testing.T, ctx context.Context)
	}{
		{
			name: "should return 401 when no AuthInfo and not in debug mode",
			setupContext: func() context.Context {
				return context.Background()
			},
			setupConfig: func() *ServerConfig {
				return &ServerConfig{
					Debug: false,
					Repos: memory.New(),
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "should return 401 when AuthInfo.Sub is empty and not in debug mode",
			setupContext: func() context.Context {
				ai := appx.AuthInfo{
					Sub: "",
				}
				return context.WithValue(context.Background(), adapter.AuthInfoKey, ai)
			},
			setupConfig: func() *ServerConfig {
				return &ServerConfig{
					Debug: false,
					Repos: memory.New(),
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "should return 401 when user not found by sub",
			setupContext: func() context.Context {
				ai := appx.AuthInfo{
					Sub: "non-existent-user",
				}
				return context.WithValue(context.Background(), adapter.AuthInfoKey, ai)
			},
			setupConfig: func() *ServerConfig {
				repos := memory.New()
				return &ServerConfig{
					Debug: false,
					Repos: repos,
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "should return 500 when repository returns unexpected error",
			setupContext: func() context.Context {
				ai := appx.AuthInfo{
					Sub: "test-sub",
				}
				return context.WithValue(context.Background(), adapter.AuthInfoKey, ai)
			},
			setupConfig: func() *ServerConfig {
				repos := memory.New()
				repos.User = &mockUserRepoWithError{err: errors.New("database error")}
				return &ServerConfig{
					Debug: false,
					Repos: repos,
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "should successfully authenticate when user exists",
			setupContext: func() context.Context {
				ai := appx.AuthInfo{
					Sub: "auth0|test-sub",
				}
				return context.WithValue(context.Background(), adapter.AuthInfoKey, ai)
			},
			setupConfig: func() *ServerConfig {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("test-user").
					Email("test@example.com").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|test-sub"}}).
					MustBuild()

				wid := workspace.NewID()
				w := workspace.New().
					ID(wid).
					Name("test-workspace").
					Members(map[id.UserID]workspace.Member{
						uid: {
							Role:      workspace.RoleOwner,
							InvitedBy: uid,
						},
					}).
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)
				repos.Workspace = memory.NewWorkspaceWith(w)

				return &ServerConfig{
					Debug: false,
					Repos: repos,
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusOK,
			assertContext: func(t *testing.T, ctx context.Context) {
				u := adapter.User(ctx)
				assert.NotNil(t, u)
				assert.Equal(t, "test-user", u.Name())
				assert.True(t, u.Auths().Has("auth0|test-sub"))

				op := adapter.Operator(ctx)
				assert.NotNil(t, op)
				assert.NotNil(t, op.User)
			},
		},
		{
			name: "should allow request in debug mode without AuthInfo",
			setupContext: func() context.Context {
				return context.Background()
			},
			setupConfig: func() *ServerConfig {
				return &ServerConfig{
					Debug: true,
					Repos: memory.New(),
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "should inject debug auth info from headers",
			setupContext: func() context.Context {
				return context.Background()
			},
			setupConfig: func() *ServerConfig {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("debug-auth-user").
					Email("debugauth@example.com").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|debug-sub"}}).
					MustBuild()

				wid := workspace.NewID()
				w := workspace.New().
					ID(wid).
					Name("debug-auth-workspace").
					Members(map[id.UserID]workspace.Member{
						uid: {
							Role:      workspace.RoleOwner,
							InvitedBy: uid,
						},
					}).
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)
				repos.Workspace = memory.NewWorkspaceWith(w)

				return &ServerConfig{
					Debug: true,
					Repos: repos,
				}
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugAuthSubHeader, "auth0|debug-sub")
				req.Header.Set(debugAuthIssHeader, "debug-iss")
				req.Header.Set(debugAuthTokenHeader, "debug-token")
				req.Header.Set(debugAuthNameHeader, "debug-name")
				req.Header.Set(debugAuthEmailHeader, "debug-email")
				return req
			},
			expectedStatus: http.StatusOK,
			assertContext: func(t *testing.T, ctx context.Context) {
				ai := adapter.GetAuthInfo(ctx)
				assert.NotNil(t, ai)
				assert.Equal(t, "auth0|debug-sub", ai.Sub)
				assert.Equal(t, "debug-iss", ai.Iss)
				assert.Equal(t, "debug-token", ai.Token)
				assert.Equal(t, "debug-name", ai.Name)
				assert.Equal(t, "debug-email", ai.Email)

				u := adapter.User(ctx)
				assert.NotNil(t, u)
				assert.Equal(t, "debug-auth-user", u.Name())
			},
		},
		{
			name: "should return 500 when generateUserOperator fails",
			setupContext: func() context.Context {
				ai := appx.AuthInfo{
					Sub: "auth0|test-sub",
				}
				return context.WithValue(context.Background(), adapter.AuthInfoKey, ai)
			},
			setupConfig: func() *ServerConfig {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("test-user").
					Email("test@example.com").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|test-sub"}}).
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)
				repos.Workspace = &mockWorkspaceRepoWithError{err: errors.New("workspace error")}

				return &ServerConfig{
					Debug: false,
					Repos: repos,
				}
			},
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			middleware := authMiddleware(cfg)

			var capturedCtx context.Context
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedCtx = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			req := tt.setupRequest()
			req = req.WithContext(tt.setupContext())
			rr := httptest.NewRecorder()

			middleware(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.assertContext != nil && capturedCtx != nil {
				tt.assertContext(t, capturedCtx)
			}
		})
	}
}

func TestAuthMiddleware_DebugUserHeader(t *testing.T) {
	uid := user.NewID()
	u := user.New().
		ID(uid).
		Name("debug-user").
		Email("debug@example.com").
		MustBuild()

	wid := workspace.NewID()
	w := workspace.New().
		ID(wid).
		Name("debug-workspace").
		Members(map[id.UserID]workspace.Member{
			uid: {
				Role:      workspace.RoleOwner,
				InvitedBy: uid,
			},
		}).
		MustBuild()

	repos := memory.New()
	repos.User = memory.NewUserWith(u)
	repos.Workspace = memory.NewWorkspaceWith(w)

	cfg := &ServerConfig{
		Debug: true,
		Repos: repos,
	}

	middleware := authMiddleware(cfg)

	var capturedCtx context.Context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(debugUserHeader, uid.String())
	rr := httptest.NewRecorder()

	middleware(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	usr := adapter.User(capturedCtx)
	assert.NotNil(t, usr)
	assert.Equal(t, "debug-user", usr.Name())
}

func TestGenerateUserOperator(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*ServerConfig, *user.User)
		wantErr bool
		assert  func(t *testing.T, op *usecase.Operator)
	}{
		{
			name: "should return nil when user is nil",
			setup: func() (*ServerConfig, *user.User) {
				return &ServerConfig{Repos: memory.New()}, nil
			},
			wantErr: false,
			assert: func(t *testing.T, op *usecase.Operator) {
				assert.Nil(t, op)
			},
		},
		{
			name: "should generate operator with workspaces",
			setup: func() (*ServerConfig, *user.User) {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("test-user").
					Email("test@example.com").
					MustBuild()

				wid1 := workspace.NewID()
				w1 := workspace.New().
					ID(wid1).
					Name("workspace1").
					Members(map[id.UserID]workspace.Member{
						uid: {
							Role:      workspace.RoleOwner,
							InvitedBy: uid,
						},
					}).
					MustBuild()

				wid2 := workspace.NewID()
				w2 := workspace.New().
					ID(wid2).
					Name("workspace2").
					Members(map[id.UserID]workspace.Member{
						uid: {
							Role:      workspace.RoleReader,
							InvitedBy: uid,
						},
					}).
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)
				repos.Workspace = memory.NewWorkspaceWith(w1, w2)

				return &ServerConfig{Repos: repos}, u
			},
			wantErr: false,
			assert: func(t *testing.T, op *usecase.Operator) {
				assert.NotNil(t, op)
				assert.NotNil(t, op.User)
				assert.Equal(t, 1, len(op.OwningWorkspaces))
				assert.Equal(t, 1, len(op.ReadableWorkspaces))
			},
		},
		{
			name: "should return error when workspace repo fails",
			setup: func() (*ServerConfig, *user.User) {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("test-user").
					Email("test@example.com").
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)
				repos.Workspace = &mockWorkspaceRepoWithError{err: errors.New("workspace error")}

				return &ServerConfig{Repos: repos}, u
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, u := tt.setup()
			op, err := generateUserOperator(context.Background(), cfg, u)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.assert != nil {
				tt.assert(t, op)
			}
		})
	}
}

func TestIsDebugUserExists(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (*http.Request, *ServerConfig)
		wantNil bool
	}{
		{
			name: "should return nil when no debug header",
			setup: func() (*http.Request, *ServerConfig) {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				return req, &ServerConfig{Repos: memory.New()}
			},
			wantNil: true,
		},
		{
			name: "should return nil when user ID is invalid",
			setup: func() (*http.Request, *ServerConfig) {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugUserHeader, "invalid-id")
				return req, &ServerConfig{Repos: memory.New()}
			},
			wantNil: true,
		},
		{
			name: "should return nil when user not found",
			setup: func() (*http.Request, *ServerConfig) {
				uid := user.NewID()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugUserHeader, uid.String())
				return req, &ServerConfig{Repos: memory.New()}
			},
			wantNil: true,
		},
		{
			name: "should return user when found",
			setup: func() (*http.Request, *ServerConfig) {
				uid := user.NewID()
				u := user.New().
					ID(uid).
					Name("debug-user").
					Email("debug@example.com").
					MustBuild()

				repos := memory.New()
				repos.User = memory.NewUserWith(u)

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugUserHeader, uid.String())
				return req, &ServerConfig{Repos: repos}
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, cfg := tt.setup()
			u := isDebugUserExists(req, cfg, context.Background())

			if tt.wantNil {
				assert.Nil(t, u)
			} else {
				assert.NotNil(t, u)
			}
		})
	}
}

func TestInjectDebugAuthInfo(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func() *http.Request
		wantNil      bool
		assertAuthIn func(t *testing.T, ai *appx.AuthInfo)
	}{
		{
			name: "should return nil when no debug auth sub header",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			wantNil: true,
		},
		{
			name: "should inject auth info from headers",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugAuthSubHeader, "test-sub")
				req.Header.Set(debugAuthIssHeader, "test-iss")
				req.Header.Set(debugAuthTokenHeader, "test-token")
				req.Header.Set(debugAuthNameHeader, "test-name")
				req.Header.Set(debugAuthEmailHeader, "test@example.com")
				return req
			},
			wantNil: false,
			assertAuthIn: func(t *testing.T, ai *appx.AuthInfo) {
				assert.Equal(t, "test-sub", ai.Sub)
				assert.Equal(t, "test-iss", ai.Iss)
				assert.Equal(t, "test-token", ai.Token)
				assert.Equal(t, "test-name", ai.Name)
				assert.Equal(t, "test@example.com", ai.Email)
			},
		},
		{
			name: "should inject auth info with only sub header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set(debugAuthSubHeader, "only-sub")
				return req
			},
			wantNil: false,
			assertAuthIn: func(t *testing.T, ai *appx.AuthInfo) {
				assert.Equal(t, "only-sub", ai.Sub)
				assert.Empty(t, ai.Iss)
				assert.Empty(t, ai.Token)
				assert.Empty(t, ai.Name)
				assert.Empty(t, ai.Email)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			ctx, ai := injectDebugAuthInfo(context.Background(), req)

			if tt.wantNil {
				assert.Nil(t, ai)
			} else {
				assert.NotNil(t, ai)
				if tt.assertAuthIn != nil {
					tt.assertAuthIn(t, ai)
				}

				// Verify context has AuthInfo
				ctxAi := adapter.GetAuthInfo(ctx)
				assert.NotNil(t, ctxAi)
				assert.Equal(t, ai.Sub, ctxAi.Sub)
			}
		})
	}
}

// Mock implementations for error testing

type mockUserRepoWithError struct {
	repo.User
	err error
}

func (m *mockUserRepoWithError) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, rerror.ErrNotFound
}

type mockWorkspaceRepoWithError struct {
	repo.Workspace
	err error
}

func (m *mockWorkspaceRepoWithError) FindByUser(ctx context.Context, uid id.UserID) (workspace.List, error) {
	return nil, m.err
}
