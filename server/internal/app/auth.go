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
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
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

	// maxBypassBodySize is the maximum request body size (in bytes) that
	// isBypassed will read. Bypass-eligible operations (signup, findByID, etc.)
	// are small by nature; rejecting oversized bodies before parsing prevents
	// unauthenticated callers from forcing large allocations.
	maxBypassBodySize = 100 * 1024 // 100 KB
)

// readCloser combines an io.Reader and an io.Closer so that
// Close() is delegated to the original request body.
type readCloser struct {
	io.Reader
	io.Closer
}

type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func isBypassed(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}

	// Read up to maxBypassBodySize+1 bytes to detect oversized bodies
	// without allocating unbounded memory.
	origBody := req.Body
	lr := io.LimitReader(origBody, maxBypassBodySize+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return false
	}
	// Restore full body for downstream handlers: concatenate what we read
	// with any remaining unread portion, preserving the original Close().
	req.Body = readCloser{
		Reader: io.MultiReader(bytes.NewReader(body), origBody),
		Closer: origBody,
	}

	if len(body) > maxBypassBodySize {
		return false
	}

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
	for field := range publicFields {
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

	// Select the operation to inspect. In GraphQL, when multiple operations
	// are present, only the one matching operationName is executed.
	var targetOp *ast.OperationDefinition
	if gqlReq.OperationName != "" {
		for _, op := range doc.Operations {
			if op.Name == gqlReq.OperationName {
				targetOp = op
				break
			}
		}
		if targetOp == nil {
			return false
		}
	} else {
		// When operationName is not set, GraphQL only allows a single operation.
		if len(doc.Operations) != 1 {
			return false
		}
		targetOp = doc.Operations[0]
	}

	// Require every top-level field in the selected operation to be @public.
	if len(targetOp.SelectionSet) == 0 {
		return false
	}
	for _, sel := range targetOp.SelectionSet {
		field, ok := sel.(*ast.Field)
		if !ok {
			return false
		}
		if _, allowed := publicFields[strings.ToLower(field.Name)]; !allowed {
			return false
		}
	}

	return true
}

func canUseDebugHeaders(cfg *ServerConfig) bool {
	return cfg.Debug || cfg.Config.Dev || os.Getenv("REEARTH_ACCOUNTS_DEV") == "true"
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

			applog.Debug(ctx, "[mockAuthMiddleware] Using mock authentication")

			// bypass some queries & mutations
			if isBypassed(req) {
				applog.Debug(ctx, "[mockAuthMiddleware] Skipping auth for signup mutation")
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
					applog.Warn(ctx, "[mockAuthMiddleware] failed to find debug user", applog.Err(err))
					usr = nil
				}
			}

			// Load demo user from database by name when no debug user is provided.
			if usr == nil {
				usr, err = cfg.Repos.User.FindByName(ctx, FIXED_MOCK_USERNAME)
				if err != nil {
					applog.Error(ctx, "[mockAuthMiddleware] failed to find demo user by name", applog.Err(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			applog.Debug(ctx, "[mockAuthMiddleware] Loaded demo user", applog.UserID(usr.ID()))

			if usr != nil {
				ctx = adapter.AttachUser(ctx, usr)
				op, err := generateUserOperator(ctx, cfg, usr)
				if err != nil {
					applog.Error(ctx, "[mockAuthMiddleware] failed to generate user operator", applog.UserID(usr.ID()), applog.Err(err))
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
				applog.Debug(ctx, "[authMiddleware] Skipping auth for signup mutation")
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}

			var usr *user.User
			var ai appx.AuthInfo

			if a, ok := ctx.Value(adapter.AuthInfoKey).(appx.AuthInfo); ok {
				ai = a
			}

			if op, err := resolveServiceOperator(ctx, cfg, &ai); err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, errServiceCallerRejected) {
					status = http.StatusUnauthorized
				}
				http.Error(w, http.StatusText(status), status)
				return
			} else if op != nil {
				next.ServeHTTP(w, req.WithContext(adapter.AttachOperator(ctx, op)))
				return
			}

			if canUseDebugHeaders(cfg) {
				if newCtx, dai := injectDebugAuthInfo(ctx, req); dai != nil {
					ctx = newCtx
					ai = *dai
					applog.Debug(ctx, "[authMiddleware] Debug auth info injected")
				}
				usr = isDebugUserExists(req, cfg, ctx)
				if usr != nil {
					applog.Debug(ctx, "[authMiddleware] Debug user header loaded", applog.UserID(usr.ID()))
				}
			}

			// Guard: Handle empty sub (only if user not already loaded via debug header)
			if usr == nil && ai.Sub == "" {
				applog.Warn(ctx, "[authMiddleware] Rejecting request with empty sub")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			// Load user by sub if not already loaded via debug header
			if usr == nil && ai.Sub != "" {
				existingUsr, err := cfg.Repos.User.FindBySub(ctx, ai.Sub)

				if err != nil {
					if errors.Is(err, rerror.ErrNotFound) {
						applog.Warn(ctx, "[authMiddleware] User not found")
						http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
						return
					}

					// Unexpected database error
					applog.Error(ctx, "[authMiddleware] Database error finding user by", applog.Err(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				usr = existingUsr
				applog.Debug(ctx, "[authMiddleware] User loaded by sub", applog.UserID(usr.ID()))
			}

			if usr != nil {
				ctx = adapter.AttachUser(ctx, usr)

				op, err := generateUserOperator(ctx, cfg, usr)
				if err != nil {
					applog.Error(ctx, "[authMiddleware] Failed to generate operator for user", applog.UserID(usr.ID()), applog.Err(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				ctx = adapter.AttachOperator(ctx, op)
				applog.Debug(ctx, "[authMiddleware] User and operator attached to context", applog.UserID(usr.ID()))
			} else {
				applog.Debug(ctx, "[authMiddleware] Proceeding without user/operator in context (debug mode)")
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
