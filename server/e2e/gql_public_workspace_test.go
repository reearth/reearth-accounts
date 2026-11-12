package e2e

import (
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
)

const publicWorkspaceFields = `{ id name alias members { name alias photoURL } }`

func anonGQL(e *httpexpect.Expect, query string, vars map[string]any) *httpexpect.Response {
	return e.POST("/api/graphql").WithJSON(GraphQLRequest{Query: query, Variables: vars}).Expect()
}

// ADR-0003 decision 2: the public view works without a token and carries no
// email, role or member user ID.
func TestPublicWorkspace_Anonymous(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, seedPrivacy)

	for name, arg := range map[string]string{
		"by id":    `id: "` + privTargetTeamWS.String() + `"`,
		"by alias": `alias: "privacy-target-team"`,
	} {
		t.Run(name, func(t *testing.T) {
			res := anonGQL(e, `{ publicWorkspace(`+arg+`) `+publicWorkspaceFields+` }`, nil).Status(http.StatusOK)
			raw := res.Body().Raw()
			ws := res.JSON().Object().Value("data").Object().Value("publicWorkspace").Object()
			ws.Value("id").String().IsEqual(privTargetTeamWS.String())
			ws.Value("name").String().IsEqual("target team")
			ws.Value("alias").String().IsEqual("privacy-target-team")
			m := ws.Value("members").Array()
			m.Length().IsEqual(1)
			m.Value(0).Object().IsEqual(map[string]any{
				"name":     privTargetName,
				"alias":    privTargetAlias,
				"photoURL": "https://target.example.com/photo.png",
			})
			assert.NotContains(t, raw, privTargetEmail)
			assert.NotContains(t, raw, privTarget.String())
		})
	}

	t.Run("the type has no email, role or user id to ask for", func(t *testing.T) {
		for _, field := range []string{"email", "role", "userId", "id", "user"} {
			anonGQL(e, `{ publicWorkspace(alias: "privacy-target-team") { members { `+field+` } } }`, nil).
				Status(http.StatusUnprocessableEntity)
		}
	})

	t.Run("unknown workspace is null", func(t *testing.T) {
		anonGQL(e, `{ publicWorkspace(alias: "no-such-workspace") { id } }`, nil).Status(http.StatusOK).
			JSON().Object().Value("data").Object().Value("publicWorkspace").IsNull()
	})

	t.Run("needs exactly one of id or alias", func(t *testing.T) {
		for _, args := range []string{"", `(id: "` + privTargetWS.String() + `", alias: "privacy-target-ws")`} {
			anonGQL(e, `{ publicWorkspace`+args+` { id } }`, nil).Status(http.StatusOK).
				JSON().Object().Value("errors").Array().Value(0).Object().
				Value("message").String().Contains("exactly one of id or alias")
		}
	})
}
