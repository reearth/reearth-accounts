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
	"github.com/reearth/reearthx/log"
	"golang.org/x/crypto/bcrypt"
)

const scimMediaType = "application/scim+json"

// ulidLen is the fixed string length of a ULID (Crockford Base32, lowercase).
const ulidLen = 26

type scimWorkspaceKey struct{}

// ScimBearerAuth returns an Echo middleware that validates a SCIM Bearer token.
//
// New tokens have the format "<workspaceID>.<secret>" and are verified with
// SHA-256 via a single FindByID — O(1) per request. Legacy tokens (no workspace-ID
// prefix, bcrypt hash) fall back to a full workspace scan; rotate tokens to
// eliminate that path.
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

			// Use fast O(1) auth for new-format tokens ("<wsID>.<secret>").
			// Only fall back to the expensive full scan for legacy tokens.
			var matched *workspace.Workspace
			if hasWorkspaceIDPrefix(token) {
				matched = scimFastAuth(ctx, workspaceRepo, token)
			} else {
				matched = scimLegacyAuth(ctx, workspaceRepo, token)
			}
			if matched == nil {
				return scimErrorResponse(c, http.StatusUnauthorized, "invalid or expired SCIM token", "")
			}

			newCtx := context.WithValue(ctx, scimWorkspaceKey{}, matched.ID())
			c.SetRequest(c.Request().WithContext(newCtx))
			return next(c)
		}
	}
}

// hasWorkspaceIDPrefix reports whether the token starts with a valid 26-character
// workspace ID (lowercase ULID) followed by a ".".
func hasWorkspaceIDPrefix(token string) bool {
	if len(token) <= ulidLen+1 || token[ulidLen] != '.' {
		return false
	}
	_, err := workspace.IDFrom(token[:ulidLen])
	return err == nil
}

// scimFastAuth authenticates a new-format token ("<workspaceID>.<secret>") with a
// single FindByID and a constant-time SHA-256 comparison. Returns nil if auth fails.
func scimFastAuth(ctx context.Context, repo workspace.Repo, token string) *workspace.Workspace {
	wsID, _ := workspace.IDFrom(token[:ulidLen])
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

// scimLegacyAuth handles pre-migration tokens (no workspace-ID prefix, bcrypt hash)
// by scanning all active workspaces. Should be eliminated once all tokens are rotated.
func scimLegacyAuth(ctx context.Context, repo workspace.Repo, token string) *workspace.Workspace {
	log.Warnfc(ctx, "scim: legacy token detected — regenerate the SCIM token to eliminate the full workspace scan")
	allWS, _, err := repo.FindAll(ctx, nil, nil, workspace.StatusActive, nil, false)
	if err != nil {
		return nil
	}
	for _, ws := range allWS {
		cfg := ws.ScimConfig()
		if cfg == nil || !cfg.Enabled() || cfg.TokenHash() == "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash()), []byte(token)) == nil {
			return ws
		}
	}
	return nil
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
