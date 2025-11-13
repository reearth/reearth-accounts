package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/pkg/id"
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
)

type graphqlRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

func isSignupMutation(req *http.Request) bool {
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

	// Check if it's a mutation
	if !strings.Contains(query, "mutation") {
		return false
	}

	// Check for signup or signupOIDC after mutation keyword
	// This handles both named and anonymous mutations
	return strings.Contains(query, "signup(") || strings.Contains(query, "signupoidc(")
}

func authMiddleware(cfg *ServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			// Skip auth for signup mutations
			if isSignupMutation(req) {
				log.Debugfc(ctx, "[authMiddleware] Skipping auth for signup mutation")
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}

			var usr *user.User
			var ai appx.AuthInfo

			if a, ok := ctx.Value(adapter.AuthInfoKey).(appx.AuthInfo); ok {
				ai = a
			}

			if cfg.Debug {
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

		return existingUsr

	}
	return nil
}

func generateUserOperator(ctx context.Context, cfg *ServerConfig, u *user.User) (*usecase.Operator, error) {
	if u == nil {
		return nil, nil
	}

	uid := u.ID()

	w, err := cfg.Repos.Workspace.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	rw := w.FilterByUserRole(uid, workspace.RoleReader).IDs()
	ww := w.FilterByUserRole(uid, workspace.RoleWriter).IDs()
	mw := w.FilterByUserRole(uid, workspace.RoleMaintainer).IDs()
	ow := w.FilterByUserRole(uid, workspace.RoleOwner).IDs()

	return &usecase.Operator{
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
