package scim

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"golang.org/x/crypto/bcrypt"
)

type scimWorkspaceKey struct{}

// ScimBearerAuth returns an Echo middleware that validates a SCIM Bearer token
// against all workspaces with SCIM enabled. The matched workspace ID is injected
// into the request context for downstream handlers.
func ScimBearerAuth(workspaceRepo workspace.Repo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Extract Bearer token from Authorization header.
			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				return scimErrorResponse(c, http.StatusUnauthorized, "missing or malformed authorization header", "")
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" {
				return scimErrorResponse(c, http.StatusUnauthorized, "missing bearer token", "")
			}

			// 2. Scan all workspaces with SCIM enabled to find the matching token.
			ctx := c.Request().Context()
			allWS, _, err := workspaceRepo.FindAll(ctx, nil, nil)
			if err != nil {
				return scimErrorResponse(c, http.StatusUnauthorized, "internal error", "")
			}

			var matched *workspace.Workspace
			for _, ws := range allWS {
				cfg := ws.ScimConfig()
				if cfg == nil || !cfg.Enabled() || cfg.TokenHash() == "" {
					continue
				}
				if err := bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash()), []byte(token)); err == nil {
					matched = ws
					break
				}
			}

			if matched == nil {
				return scimErrorResponse(c, http.StatusUnauthorized, "invalid or expired SCIM token", "")
			}

			// 3. Inject workspace ID into context.
			newCtx := context.WithValue(ctx, scimWorkspaceKey{}, matched.ID())
			c.SetRequest(c.Request().WithContext(newCtx))
			return next(c)
		}
	}
}

// WorkspaceIDFromContext retrieves the authenticated workspace ID injected by ScimBearerAuth.
func WorkspaceIDFromContext(ctx context.Context) (workspace.ID, bool) {
	v := ctx.Value(scimWorkspaceKey{})
	if v == nil {
		return workspace.ID{}, false
	}
	id, ok := v.(workspace.ID)
	return id, ok
}

// scimErrorResponse writes a JSON ScimError response body and returns nil (the response is
// already committed). Callers should return the result of this function.
func scimErrorResponse(c echo.Context, status int, detail, scimType string) error {
	return c.JSON(status, ScimError{
		Detail:   detail,
		Schemas:  []string{ScimSchemaError},
		ScimType: scimType,
		Status:   fmt.Sprintf("%d", status),
	})
}
