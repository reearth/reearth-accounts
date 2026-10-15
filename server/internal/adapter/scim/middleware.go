package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

const scimMediaType = "application/scim+json"

// ulidLen is the fixed string length of a ULID (Crockford Base32, lowercase).
const ulidLen = 26

type scimWorkspaceKey struct{}

// ScimBearerAuth returns an Echo middleware that validates a SCIM Bearer token.
// Tokens must have the format "<workspaceID>.<secret>" produced by GenerateScimToken;
// the middleware performs a single FindByID and a SHA-256 comparison — O(1) per request.
func ScimBearerAuth(workspaceRepo workspace.Repo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				return scimErrorResponse(c, http.StatusUnauthorized, "missing or malformed authorization header", "")
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" {
				return scimErrorResponse(c, http.StatusUnauthorized, "missing bearer token", "")
			}

			ctx := c.Request().Context()
			ws := authenticateScimToken(ctx, workspaceRepo, token)
			if ws == nil {
				return scimErrorResponse(c, http.StatusUnauthorized, "invalid or expired SCIM token", "")
			}

			newCtx := context.WithValue(ctx, scimWorkspaceKey{}, ws.ID())
			c.SetRequest(c.Request().WithContext(newCtx))
			return next(c)
		}
	}
}

// authenticateScimToken validates a token in "<workspaceID>.<secret>" format:
// parses the workspace ID prefix, fetches that workspace, and verifies the
// SHA-256 hash of the full token against the stored hash.
func authenticateScimToken(ctx context.Context, repo workspace.Repo, token string) *workspace.Workspace {
	if len(token) <= ulidLen+1 || token[ulidLen] != '.' {
		return nil
	}
	wsID, err := workspace.IDFrom(token[:ulidLen])
	if err != nil {
		return nil
	}
	ws, err := repo.FindByID(ctx, wsID)
	if err != nil {
		return nil
	}
	cfg := ws.ScimConfig()
	if cfg == nil || !cfg.Enabled() || cfg.TokenHash() == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(token))
	if hex.EncodeToString(sum[:]) != cfg.TokenHash() {
		return nil
	}
	return ws
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

// scimErrorResponse writes a SCIM error body with Content-Type: application/scim+json.
func scimErrorResponse(c echo.Context, status int, detail, scimType string) error {
	b, _ := json.Marshal(ScimError{
		Detail:   detail,
		Schemas:  []string{ScimSchemaError},
		ScimType: scimType,
		Status:   fmt.Sprintf("%d", status),
	})
	return c.Blob(status, scimMediaType, b)
}

// scimJSON writes a SCIM response body with Content-Type: application/scim+json.
func scimJSON(c echo.Context, status int, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}
	return c.Blob(status, scimMediaType, b)
}
