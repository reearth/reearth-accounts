package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/auth0"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

const auth0Sub = "auth0|e2etestuser"

func seedAuth0User(ctx context.Context, r *repo.Container) error {
	if err := seedRoles(ctx, r); err != nil {
		return err
	}

	auth := user.Auth{Provider: user.ProviderAuth0, Sub: auth0Sub}
	u := user.New().ID(uId).
		Name("auth0user").
		Email("auth0@e2e.com").
		Auths([]user.Auth{auth}).
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}

	roleOwner := workspace.Member{
		Role:      role.RoleOwner,
		InvitedBy: uId,
	}
	w := workspace.New().ID(wId).
		Name("auth0user").
		Personal(true).
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: roleOwner,
		}).
		MustBuild()
	return r.Workspace.Save(ctx, w)
}

func newMockAuth0Server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "mock-token",
				"expires_in":   86400,
				"scope":        "read:users update:users",
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2/users/"+auth0Sub:
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			name := "auth0user"
			if n, ok := body["name"].(string); ok {
				name = n
			}
			email := "auth0@e2e.com"
			if e, ok := body["email"].(string); ok {
				email = e
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"user_id": auth0Sub,
				"name":    name,
				"email":   email,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestUpdateMe_Auth0User_UpdatesName(t *testing.T) {
	mockSrv := newMockAuth0Server(t)
	defer mockSrv.Close()

	authenticator := auth0.New(mockSrv.URL, "clientid", "clientsecret", 0)
	e, _ := StartServerWithAuthenticator(t, &app.Config{}, true, seedAuth0User, authenticator)

	query := `mutation { updateMe(input: {name: "renameduser"}){ me{ id name } }}`
	request := GraphQLRequest{Query: query}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusOK).
		JSON().Object().Value("data").Object().Value("updateMe").Object().Value("me").Object()
	o.Value("name").String().IsEqual("renameduser")
}
