package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	t.Run("should return 401 when no AuthInfo and not in debug mode", func(t *testing.T) {
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.Background())
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 401 when AuthInfo.Sub is empty and not in debug mode", func(t *testing.T) {
		ai := appx.AuthInfo{
			Sub: "",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 401 when user not found by sub", func(t *testing.T) {
		ai := appx.AuthInfo{
			Sub: "non-existent-user",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		repos := memory.New()
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 500 when repository returns unexpected error", func(t *testing.T) {
		ai := appx.AuthInfo{
			Sub: "test-sub",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		repos := memory.New()
		repos.User = &mockUserRepoWithError{err: errors.New("database error")}
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("should successfully authenticate when user exists", func(t *testing.T) {
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
					Role:      role.RoleOwner,
					InvitedBy: uid,
				},
			}).
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(u)
		repos.Workspace = memory.NewWorkspaceWith(w)

		ai := appx.AuthInfo{
			Sub: "auth0|test-sub",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		var capturedCtx context.Context
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		usr := adapter.User(capturedCtx)
		assert.NotNil(t, usr)
		assert.Equal(t, "test-user", usr.Name())
		assert.True(t, usr.Auths().Has("auth0|test-sub"))

		op := adapter.Operator(capturedCtx)
		assert.NotNil(t, op)
		assert.NotNil(t, op.User)
	})

	t.Run("should allow request in debug mode without AuthInfo", func(t *testing.T) {
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: true,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.Background())
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should allow request in debug mode when user not found by sub (for signup)", func(t *testing.T) {
		ai := appx.AuthInfo{
			Sub: "auth0|new-user-sub",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: true,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		var capturedCtx context.Context
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// User should be nil since they don't exist yet
		usr := adapter.User(capturedCtx)
		assert.Nil(t, usr)

		// Operator should also be nil
		op := adapter.Operator(capturedCtx)
		assert.Nil(t, op)
	})

	t.Run("should inject debug auth info from headers", func(t *testing.T) {
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
					Role:      role.RoleOwner,
					InvitedBy: uid,
				},
			}).
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(u)
		repos.Workspace = memory.NewWorkspaceWith(w)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
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
		req.Header.Set(debugAuthSubHeader, "auth0|debug-sub")
		req.Header.Set(debugAuthIssHeader, "debug-iss")
		req.Header.Set(debugAuthTokenHeader, "debug-token")
		req.Header.Set(debugAuthNameHeader, "debug-name")
		req.Header.Set(debugAuthEmailHeader, "debug-email")
		req = req.WithContext(context.Background())
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		ai := adapter.GetAuthInfo(capturedCtx)
		assert.NotNil(t, ai)
		assert.Equal(t, "auth0|debug-sub", ai.Sub)
		assert.Equal(t, "debug-iss", ai.Iss)
		assert.Equal(t, "debug-token", ai.Token)
		assert.Equal(t, "debug-name", ai.Name)
		assert.Equal(t, "debug-email", ai.Email)

		usr := adapter.User(capturedCtx)
		assert.NotNil(t, usr)
		assert.Equal(t, "debug-auth-user", usr.Name())
	})

	t.Run("should return 500 when generateUserOperator fails", func(t *testing.T) {
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

		ai := appx.AuthInfo{
			Sub: "auth0|test-sub",
		}
		ctx := context.WithValue(context.Background(), adapter.AuthInfoKey, ai)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: false,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestMockAuthMiddleware(t *testing.T) {
	t.Run("should skip auth for signup mutation", func(t *testing.T) {
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: true,
			},
			Debug: false,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		body := `{"query":"mutation { signup(input: {}) { id } }","operationName":"signup"}`
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should return 500 when demo user not found", func(t *testing.T) {
		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: true,
			},
			Debug: false,
			Repos: memory.New(),
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("should successfully authenticate with demo user", func(t *testing.T) {
		uid := user.NewID()
		demoUser := user.New().
			ID(uid).
			Name("Demo user").
			Email("demo@example.com").
			MustBuild()

		wid := workspace.NewID()
		w := workspace.New().
			ID(wid).
			Name("demo-workspace").
			Members(map[id.UserID]workspace.Member{
				uid: {
					Role:      role.RoleOwner,
					InvitedBy: uid,
				},
			}).
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(demoUser)
		repos.Workspace = memory.NewWorkspaceWith(w)

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: true,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		var capturedCtx context.Context
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		usr := adapter.User(capturedCtx)
		assert.NotNil(t, usr)
		assert.Equal(t, "Demo user", usr.Name())

		op := adapter.Operator(capturedCtx)
		assert.NotNil(t, op)
		assert.NotNil(t, op.User)
	})

	t.Run("should return 500 when generateUserOperator fails", func(t *testing.T) {
		uid := user.NewID()
		demoUser := user.New().
			ID(uid).
			Name("Demo user").
			Email("demo@example.com").
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(demoUser)
		repos.Workspace = &mockWorkspaceRepoWithError{err: errors.New("workspace error")}

		cfg := &ServerConfig{
			Config: &Config{
				Mock_Auth: true,
			},
			Debug: false,
			Repos: repos,
		}
		middleware := authMiddleware(cfg)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		middleware(nextHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
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
				Role:      role.RoleOwner,
				InvitedBy: uid,
			},
		}).
		MustBuild()

	repos := memory.New()
	repos.User = memory.NewUserWith(u)
	repos.Workspace = memory.NewWorkspaceWith(w)

	cfg := &ServerConfig{
		Config: &Config{
			Mock_Auth: false,
		},
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
	t.Run("should return nil when user is nil", func(t *testing.T) {
		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  memory.New(),
		}
		op, err := generateUserOperator(context.Background(), cfg, nil)

		assert.NoError(t, err)
		assert.Nil(t, op)
	})

	t.Run("should generate operator with workspaces", func(t *testing.T) {
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
					Role:      role.RoleOwner,
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
					Role:      role.RoleReader,
					InvitedBy: uid,
				},
			}).
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(u)
		repos.Workspace = memory.NewWorkspaceWith(w1, w2)

		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  repos,
		}
		op, err := generateUserOperator(context.Background(), cfg, u)

		assert.NoError(t, err)
		assert.NotNil(t, op)
		assert.NotNil(t, op.User)
		assert.Equal(t, 1, len(op.OwningWorkspaces))
		assert.Equal(t, 1, len(op.ReadableWorkspaces))
	})

	t.Run("should return error when workspace repo fails", func(t *testing.T) {
		uid := user.NewID()
		u := user.New().
			ID(uid).
			Name("test-user").
			Email("test@example.com").
			MustBuild()

		repos := memory.New()
		repos.User = memory.NewUserWith(u)
		repos.Workspace = &mockWorkspaceRepoWithError{err: errors.New("workspace error")}

		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  repos,
		}
		op, err := generateUserOperator(context.Background(), cfg, u)

		assert.Error(t, err)
		assert.Nil(t, op)
	})
}

func TestIsDebugUserExists(t *testing.T) {
	t.Run("should return nil when no debug header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  memory.New(),
		}
		u := isDebugUserExists(req, cfg, context.Background())

		assert.Nil(t, u)
	})

	t.Run("should return nil when user ID is invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(debugUserHeader, "invalid-id")
		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  memory.New(),
		}
		u := isDebugUserExists(req, cfg, context.Background())

		assert.Nil(t, u)
	})

	t.Run("should return nil when user not found", func(t *testing.T) {
		uid := user.NewID()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(debugUserHeader, uid.String())
		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  memory.New(),
		}
		u := isDebugUserExists(req, cfg, context.Background())

		assert.Nil(t, u)
	})

	t.Run("should return user when found", func(t *testing.T) {
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
		cfg := &ServerConfig{
			Config: &Config{},
			Repos:  repos,
		}
		foundUser := isDebugUserExists(req, cfg, context.Background())

		assert.NotNil(t, foundUser)
		assert.Equal(t, "debug-user", foundUser.Name())
	})
}

func TestInjectDebugAuthInfo(t *testing.T) {
	t.Run("should return nil when no debug auth sub header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx, ai := injectDebugAuthInfo(context.Background(), req)

		assert.Nil(t, ai)
		assert.NotNil(t, ctx)
	})

	t.Run("should inject auth info from headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(debugAuthSubHeader, "test-sub")
		req.Header.Set(debugAuthIssHeader, "test-iss")
		req.Header.Set(debugAuthTokenHeader, "test-token")
		req.Header.Set(debugAuthNameHeader, "test-name")
		req.Header.Set(debugAuthEmailHeader, "test@example.com")

		ctx, ai := injectDebugAuthInfo(context.Background(), req)

		assert.NotNil(t, ai)
		assert.Equal(t, "test-sub", ai.Sub)
		assert.Equal(t, "test-iss", ai.Iss)
		assert.Equal(t, "test-token", ai.Token)
		assert.Equal(t, "test-name", ai.Name)
		assert.Equal(t, "test@example.com", ai.Email)

		// Verify context has AuthInfo
		ctxAi := adapter.GetAuthInfo(ctx)
		assert.NotNil(t, ctxAi)
		assert.Equal(t, ai.Sub, ctxAi.Sub)
	})

	t.Run("should inject auth info with only sub header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(debugAuthSubHeader, "only-sub")

		_, ai := injectDebugAuthInfo(context.Background(), req)

		assert.NotNil(t, ai)
		assert.Equal(t, "only-sub", ai.Sub)
		assert.Empty(t, ai.Iss)
		assert.Empty(t, ai.Token)
		assert.Empty(t, ai.Name)
		assert.Empty(t, ai.Email)
	})
}

// Mock implementations for error testing

type mockUserRepoWithError struct {
	user.Repo
	err error
}

func (m *mockUserRepoWithError) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, rerror.ErrNotFound
}

type mockWorkspaceRepoWithError struct {
	workspace.Repo
	err error
}

func (m *mockWorkspaceRepoWithError) FindByUser(ctx context.Context, uid id.UserID) (workspace.List, error) {
	return nil, m.err
}
