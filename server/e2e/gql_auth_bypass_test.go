package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

func baseSeederAuthBypass(ctx context.Context, r *repo.Container) error {
	auth := user.ReearthSub(uId.String())

	u := user.New().ID(uId).
		Name("target-user").
		Email("target@example.com").
		Auths([]user.Auth{*auth}).
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}

	w := workspace.New().ID(wId).
		Name("target-workspace").
		Personal(true).
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: {Role: role.RoleOwner, InvitedBy: uId},
		}).
		MustBuild()
	return r.Workspace.Save(ctx, w)
}

// TestSEC04_XInternalServiceHeaderDoesNotBypassAuth verifies that
// X-Internal-Service: visualizer-api cannot be used to authenticate
// without a valid JWT when the server runs in production mode (Debug: false).
func TestSEC04_XInternalServiceHeaderDoesNotBypassAuth(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, baseSeederAuthBypass)

	query := `{ me { id name email } }`
	request := GraphQLRequest{Query: query}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	// Sending X-Internal-Service + X-Reearth-Debug-User without a valid JWT.
	// Before the fix this would have authenticated as the target user.
	// After the fix the server must reject with 401.
	e.POST("/api/graphql").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Internal-Service", "visualizer-api").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusUnauthorized)
}

// TestSEC04_XInternalServiceWithDebugAuthSubDoesNotBypassAuth verifies that
// X-Internal-Service: visualizer-api + X-Reearth-Debug-Auth-Sub cannot inject
// arbitrary auth info without a valid JWT in production mode.
func TestSEC04_XInternalServiceWithDebugAuthSubDoesNotBypassAuth(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, baseSeederAuthBypass)

	query := `{ me { id name email } }`
	request := GraphQLRequest{Query: query}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	e.POST("/api/graphql").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Internal-Service", "visualizer-api").
		WithHeader("X-Reearth-Debug-Auth-Sub", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusUnauthorized)
}

// TestSEC04_DebugHeadersWorkInDebugMode confirms that debug authentication
// still functions correctly when the server is explicitly started with Debug: true.
func TestSEC04_DebugHeadersWorkInDebugMode(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, false, baseSeederAuthBypass)

	query := `{ me { id name email } }`
	request := GraphQLRequest{Query: query}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusOK).
		JSON().Object().
		Value("data").Object().
		Value("me").Object()

	o.Value("id").String().IsEqual(uId.String())
	o.Value("name").String().IsEqual("target-user")
	o.Value("email").String().IsEqual("target@example.com")
}
