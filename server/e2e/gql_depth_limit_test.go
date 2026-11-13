package e2e

import (
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/workspace"
	"github.com/stretchr/testify/require"
)

// The deepest query any pkg/gqlclient call sends (depth 5).
func deepestClientQuery(t *testing.T) GraphQLRequest {
	t.Helper()
	vars := map[string]any{"userId": graphql.ID(uId.String()), "page": graphql.Int(1), "size": graphql.Int(10)}
	q, err := graphql.ConstructQuery(&workspace.FindByUserWithPaginationQuery{}, vars)
	require.NoError(t, err)
	return GraphQLRequest{Query: q, Variables: map[string]any{"userId": uId.String(), "page": 1, "size": 10}}
}

// Depth 6: me > myWorkspace > members > user > metadata > lang.
var depthSixQuery = GraphQLRequest{Query: `{ me { myWorkspace { members { ... on WorkspaceUserMember { user { metadata { lang } } } } } } }`}

func gqlAsSeededUser(e *httpexpect.Expect, req GraphQLRequest) *httpexpect.Object {
	return e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithJSON(req).
		Expect().JSON().Object()
}

func TestGQLDepthLimit(t *testing.T) {
	t.Setenv("SKIP_DOTENV", "1")
	defaults, err := app.ReadConfig(false)
	require.NoError(t, err)

	t.Run("deepest client query passes the default limit", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{GraphQL: defaults.GraphQL}, false, baseSeederAuthBypass)
		res := gqlAsSeededUser(e, deepestClientQuery(t))
		res.NotContainsKey("errors")
		res.Value("data").Object().Value("findByUserWithPagination").Object().Value("totalCount").Number().IsEqual(1)
	})

	t.Run("limit is inclusive and one level past it is rejected", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{GraphQL: app.GraphQLConfig{DepthLimit: 5}}, false, baseSeederAuthBypass)

		gqlAsSeededUser(e, deepestClientQuery(t)).NotContainsKey("errors")

		res := gqlAsSeededUser(e, depthSixQuery)
		res.Value("errors").Array().Value(0).Object().
			Value("extensions").Object().Value("code").String().IsEqual("DEPTH_LIMIT_EXCEEDED")

		viaFragment := GraphQLRequest{Query: `{ me { myWorkspace { ...W } } } fragment W on Workspace { members { ... on WorkspaceUserMember { user { metadata { lang } } } } }`}
		gqlAsSeededUser(e, viaFragment).Value("errors").Array().Value(0).Object().
			Value("extensions").Object().Value("code").String().IsEqual("DEPTH_LIMIT_EXCEEDED")
	})
}
