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
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
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

// bypassedFields is the set of top-level GraphQL field names that are
// allowed without authentication. Field names must be lowercase.
var bypassedFields = map[string]struct{}{
	"authconfig":                   {},
	"createverification":           {},
	"findbyalias":                  {},
	"findbyid":                     {},
	"findbyids":                    {},
	"findusersbyidswithpagination": {},
	"signup":                       {},
	"signupoidc":                   {},
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

	if gqlReq.Query == "" {
		return false
	}

	// Cheap prefilter: skip AST parsing if the raw query doesn't contain
	// any of the bypassed field names. This avoids parsing overhead for
	// the majority of authenticated requests.
	queryLower := strings.ToLower(gqlReq.Query)
	hasCandidate := false
	for field := range bypassedFields {
		if strings.Contains(queryLower, field) {
			hasCandidate = true
			break
		}
	}
	if !hasCandidate {
		return false
	}

	doc, gqlErr := parser.ParseQuery(&ast.Source{Input: gqlReq.Query})
	if gqlErr != nil {
		return false
	}

	// A GraphQL document can contain multiple named operations (e.g. query A {...} mutation B {...}).
	// Only one is executed per request, selected by the operationName field. We iterate all
	// operations and require every top-level field across all of them to be in the bypass list.
	// This is intentionally conservative: even though only one operation runs, we reject the
	// request if any operation contains a non-bypassed field.
	fieldCount := 0
	for _, op := range doc.Operations {
		for _, sel := range op.SelectionSet {
			field, ok := sel.(*ast.Field)
			if !ok {
				return false
			}
			if _, allowed := bypassedFields[strings.ToLower(field.Name)]; !allowed {
				return false
			}
			fieldCount++
		}
	}

	return fieldCount > 0
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
					log.Debugfc(ctx, "[authMiddleware] Debug auth info injected: sub=%s", ai.Sub)
				}
				usr = isDebugUserExists(req, cfg, ctx)
				if usr != nil {
					log.Debugfc(ctx, "[authMiddleware] Debug user header loaded: %s (%s)", usr.Name(), usr.ID())
				}
			}

			// Guard: Handle empty sub (only if user not already loaded via debug header)
			if usr == nil && ai.Sub == "" {
				log.Warnfc(ctx, "[authMiddleware] Rejecting request with empty sub")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			// Load user by sub if not already loaded via debug header
			if usr == nil && ai.Sub != "" {
				existingUsr, err := cfg.Repos.User.FindBySub(ctx, ai.Sub)

				if err != nil {
					if errors.Is(err, rerror.ErrNotFound) {
						log.Warnfc(ctx, "[authMiddleware] User not found for sub=%s", ai.Sub)
						http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
						return
					}

					// Unexpected database error
					log.Errorfc(ctx, "[authMiddleware] Database error finding user by sub=%s: %v", ai.Sub, err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				usr = existingUsr
				log.Debugfc(ctx, "[authMiddleware] User loaded by sub: %s (%s)", usr.Name(), usr.ID())
			}

			if usr != nil {
				ctx = adapter.AttachUser(ctx, usr)

				op, err := generateUserOperator(ctx, cfg, usr)
				if err != nil {
					log.Errorfc(ctx, "[authMiddleware] Failed to generate operator for user %s: %v", usr.ID(), err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				ctx = adapter.AttachOperator(ctx, op)
				log.Debugfc(ctx, "[authMiddleware] User and operator attached to context: %s", usr.ID())
			} else {
				log.Debugfc(ctx, "[authMiddleware] Proceeding without user/operator in context (debug mode)")
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
