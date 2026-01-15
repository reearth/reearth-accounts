package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

const (
	debugUserHeader      = "X-Reearth-Debug-User"
	debugAuthSubHeader   = "X-Reearth-Debug-Auth-Sub"
	debugAuthIssHeader   = "X-Reearth-Debug-Auth-Iss"
	debugAuthTokenHeader = "X-Reearth-Debug-Auth-Token"
	debugAuthNameHeader  = "X-Reearth-Debug-Auth-Name"
	debugAuthEmailHeader = "X-Reearth-Debug-Auth-Email"
	FIXED_MOCK_USERNAME  = "Demo User"
	FIXED_MOCK_USERMAILE = "demo@example.com"
)

type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func isBypassed(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return false
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	var gqlReq graphqlRequest
	if err := json.Unmarshal(body, &gqlReq); err != nil {
		return false
	}

	query := strings.ToLower(gqlReq.Query)
	query = strings.ReplaceAll(query, " ", "")
	query = strings.ReplaceAll(query, "\n", "")
	query = strings.ReplaceAll(query, "\t", "")
	query = strings.ReplaceAll(query, "\r", "")

	list := []string{
		"signup(",
		"signupoidc(",
		"findbyid(",
		"findbyalias(",
		"createverification(",
	}

	for _, q := range list {
		if strings.Contains(query, q) {
			return true
		}
	}

	return false
}

func canUseDebugHeaders(req *http.Request, cfg *ServerConfig) bool {
	if cfg.Debug || cfg.Config.Dev || os.Getenv("REEARTH_ACCOUNTS_DEV") == "true" {
		return true
	}
	if req.Header.Get("X-Internal-Service") == "visualizer-api" {
		return true
	}
	return false
}

func authMiddleware(cfg *ServerConfig) func(http.Handler) http.Handler {
	if cfg.Config.Mock_Auth {
		return mockAuthMiddleware(cfg)
	}
	return identityProviderAuthMiddleware(cfg)
}

func mockAuthMiddleware(cfg *ServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			log.Debugfc(ctx, "[mockAuthMiddleware] Using mock authentication")

			// bypass some queries & mutations
			if isBypassed(req) {
				log.Debugfc(ctx, "[mockAuthMiddleware] Skipping auth for signup mutation")
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}

			var usr *user.User
			var err error

			// Allow selecting a specific user via debug header.
			if debugUser := req.Header.Get(debugUserHeader); debugUser != "" {
				if uID, idErr := id.UserIDFrom(debugUser); idErr == nil {
					usr, err = cfg.Repos.User.FindByID(ctx, uID)
				} else {
					usr, err = cfg.Repos.User.FindByName(ctx, debugUser)
				}
				if err != nil {
					log.Warnfc(ctx, "[mockAuthMiddleware] failed to find debug user: %s, error: %s", debugUser, err.Error())
					usr = nil
				}
			}

			// Load demo user from database by name when no debug user is provided.
			if usr == nil {
				usr, err = cfg.Repos.User.FindByName(ctx, FIXED_MOCK_USERNAME)
				if err != nil {
					log.Errorfc(ctx, "[mockAuthMiddleware] failed to find demo user by name: %s, error: %s", FIXED_MOCK_USERNAME, err.Error())
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			log.Debugfc(ctx, "[mockAuthMiddleware] Loaded demo user: %s (%s)", usr.Name(), usr.ID())

			if usr != nil {
				ctx = adapter.AttachUser(ctx, usr)
				op, err := generateUserOperator(ctx, cfg, usr)
				if err != nil {
					log.Errorfc(ctx, "[mockAuthMiddleware] failed to generate user operator: %s, error: %s", usr.ID(), err.Error())
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				ctx = adapter.AttachOperator(ctx, op)
			}

			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func identityProviderAuthMiddleware(cfg *ServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// bypass some queries & mutations
			if isBypassed(req) {
				log.Debugfc(ctx, "[authMiddleware] Skipping auth for signup mutation")
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}

			var usr *user.User
			var ai appx.AuthInfo

			if a, ok := ctx.Value(adapter.AuthInfoKey).(appx.AuthInfo); ok {
				ai = a
			}

			if canUseDebugHeaders(req, cfg) {
				if newCtx, dai := injectDebugAuthInfo(ctx, req); dai != nil {
					ctx = newCtx
					ai = *dai
				}
				usr = isDebugUserExists(req, cfg, ctx)
			}

			if usr == nil {
				if ai.Sub == "" && !cfg.Debug {
					log.Warnfc(ctx, "[authMiddleware] sub is empty and debug is disabled")
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				if ai.Sub != "" {
					existingUsr, err := cfg.Repos.User.FindBySub(ctx, ai.Sub)
					if err != nil {
						if errors.Is(err, rerror.ErrNotFound) {
							// In debug mode, allow requests without an existing user (e.g., for signup)
							if cfg.Debug {
								log.Debugfc(ctx, "[authMiddleware] user not found by sub in debug mode, allowing request: %s", ai.Sub)
							} else {
								log.Warnfc(ctx, "[authMiddleware] failed to find user by sub: %s, error: %s", ai.Sub, err.Error())
								http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
								return
							}
						} else {
							log.Errorfc(ctx, "[authMiddleware] failed to find user by sub: %s, error: %s", ai.Sub, err.Error())
							http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
							return
						}
					} else {
						usr = existingUsr
					}
				} else {
					log.Errorfc(ctx, "[authMiddleware] sub is empty")
				}
			}

			if usr != nil {
				ctx = adapter.AttachUser(ctx, usr)
				op, err := generateUserOperator(ctx, cfg, usr)
				if err != nil {
					log.Errorfc(ctx, "[authMiddleware] failed to generate user operator: %s, error: %s", usr.ID(), err.Error())
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				ctx = adapter.AttachOperator(ctx, op)
			}

			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func isDebugUserExists(req *http.Request, cfg *ServerConfig, ctx context.Context) *user.User {
	if userID := req.Header.Get(debugUserHeader); userID != "" {
		var existingUsr *user.User

		if uID, err := id.UserIDFrom(userID); err == nil {
			u, err := cfg.Repos.User.FindByID(ctx, uID)
			if err == nil {
				existingUsr = u
			}
		}

		if existingUsr == nil && strings.Contains(userID, "@") {
			u, err := cfg.Repos.User.FindByEmail(ctx, userID)
			if err == nil {
				existingUsr = u
			}
		}

		if existingUsr == nil {
			u, err := cfg.Repos.User.FindByName(ctx, userID)
			if err == nil {
				existingUsr = u
			}
		}

		return existingUsr

	}
	return nil
}

func generateUserOperator(ctx context.Context, cfg *ServerConfig, u *user.User) (*workspace.Operator, error) {
	if u == nil {
		return nil, nil
	}

	uid := u.ID()

	w, err := cfg.Repos.Workspace.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	rw := w.FilterByUserRole(uid, role.RoleReader).IDs()
	ww := w.FilterByUserRole(uid, role.RoleWriter).IDs()
	mw := w.FilterByUserRole(uid, role.RoleMaintainer).IDs()
	ow := w.FilterByUserRole(uid, role.RoleOwner).IDs()

	return &workspace.Operator{
		User: &uid,

		ReadableWorkspaces:     rw,
		WritableWorkspaces:     ww,
		MaintainableWorkspaces: mw,
		OwningWorkspaces:       ow,
	}, nil
}

func injectDebugAuthInfo(ctx context.Context, req *http.Request) (context.Context, *appx.AuthInfo) {
	sub := req.Header.Get(debugAuthSubHeader)
	if sub == "" {
		return ctx, nil
	}
	ai := &appx.AuthInfo{
		Token: req.Header.Get(debugAuthTokenHeader),
		Sub:   sub,
		Iss:   req.Header.Get(debugAuthIssHeader),
		Name:  req.Header.Get(debugAuthNameHeader),
		Email: req.Header.Get(debugAuthEmailHeader),
	}
	return context.WithValue(ctx, adapter.AuthInfoKey, *ai), ai
}
