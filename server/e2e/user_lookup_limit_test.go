package e2e

import (
	"net/http"
	"strings"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
)

func lookupLimitServer(t *testing.T, enforce bool) *httpexpect.Expect {
	t.Helper()
	e, _ := StartServer(t, &app.Config{
		Mock_Auth:       true,
		UserLookupLimit: ratelimit.Config{Enforce: enforce, PerMinute: 1, Burst: 2},
	}, false, seedDemoUser)
	return e
}

func gqlLookup(e *httpexpect.Expect, asUser string, query string) *httpexpect.Response {
	r := e.POST("/api/graphql").WithJSON(GraphQLRequest{Query: query})
	if asUser != "" {
		r = r.WithHeader("X-Reearth-Debug-User", asUser)
	}
	return r.Expect()
}

func TestUserLookupLimit_GraphQL(t *testing.T) {
	lookup := `{ findUserByAlias(alias: "other-rest-user") { id } }`

	t.Run("enforce mode answers 429 once the viewer's budget is spent", func(t *testing.T) {
		e := lookupLimitServer(t, true)
		gqlLookup(e, "", lookup).Status(http.StatusOK)
		gqlLookup(e, "", lookup).Status(http.StatusOK)
		res := gqlLookup(e, "", lookup).Status(http.StatusTooManyRequests).JSON().Object()
		res.Value("errors").Array().Value(0).Object().
			Value("extensions").Object().Value("code").String().IsEqual("RATE_LIMITED")

		gqlLookup(e, restOtherUID.String(), lookup).Status(http.StatusOK)
	})

	t.Run("aliases in one operation each cost a lookup", func(t *testing.T) {
		e := lookupLimitServer(t, true)
		gqlLookup(e, "", `{ a: findUserByAlias(alias: "x") { id } b: findUserByAlias(alias: "y") { id } c: searchUser(keyword: "zz") { id } }`).
			Status(http.StatusTooManyRequests)
	})

	t.Run("non-lookup queries are not charged", func(t *testing.T) {
		e := lookupLimitServer(t, true)
		for range 5 {
			gqlLookup(e, "", `{ me { id } }`).Status(http.StatusOK)
		}
	})

	t.Run("log-only mode allows over-budget lookups", func(t *testing.T) {
		e := lookupLimitServer(t, false)
		for range 4 {
			gqlLookup(e, "", lookup).Status(http.StatusOK).JSON().Object().NotContainsKey("errors")
		}
	})
}

func TestUserLookupLimit_REST(t *testing.T) {
	routes := []string{
		"/api/users/search?keyword=other-rest-user",
		"/api/users/by-alias?alias=other-rest-user",
		"/api/users/by-name-or-email?q=other-rest-user",
		"/api/users/by-name-or-alias?q=other-rest-user",
		"/api/users?ids=" + restOtherUID.String(),
		"/api/users/" + restOtherUID.String(),
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			e := lookupLimitServer(t, true)
			path, query, _ := strings.Cut(route, "?")
			get := func() *httpexpect.Response { return e.GET(path).WithQueryString(query).Expect() }
			for range 2 {
				assert.NotEqual(t, http.StatusTooManyRequests, get().Raw().StatusCode)
			}
			get().Status(http.StatusTooManyRequests)
		})
	}

	t.Run("log-only mode allows over-budget lookups", func(t *testing.T) {
		e := lookupLimitServer(t, false)
		for range 4 {
			e.GET("/api/users/" + restOtherUID.String()).Expect().Status(http.StatusOK)
		}
	})
}
