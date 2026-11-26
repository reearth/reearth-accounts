package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/require"
)

func TestUserLookupLimit_GraphQLCountsAClientCallAgainstTheClient(t *testing.T) {
	anonLookup := `{ findUsersByIDsWithPagination(ids: ["` + privTarget.String() + `"], pagination: {page: 1, size: 1}) { totalCount } }`
	intLookup := `{ node(id: "` + auShared.String() + `", type: WORKSPACE) { id } }`
	userLookup := `{ me { id } findUsersByIDsWithPagination(ids: ["` + privTarget.String() + `"], pagination: {page: 1, size: 1}) { totalCount } }`
	asInt := "integration:" + svcIntegration.String()

	start := func(t *testing.T) (*httpexpect.Expect, *svcIdP) {
		idp := newSvcIdP(t)
		jwks := idp.srv.URL + "/jwks.json"
		cfg := &app.Config{
			Auth:            app.AuthConfigs{{ISS: idp.issuer(), AUD: []string{svcAudience}, JWKSURI: &jwks}},
			UserLookupLimit: ratelimit.Config{Enforce: true, PerMinute: 1, Burst: 1},
		}
		callers, err := json.Marshal([]map[string]any{
			{"issuer": idp.issuer(), "client_id": "svc-a", "name": "a-app", "act_as": []string{"integration"}},
			{"issuer": idp.issuer(), "client_id": "svc-b", "name": "b-app", "act_as": []string{"integration"}},
		})
		require.NoError(t, err)
		require.NoError(t, cfg.ServiceCallers.Decode(string(callers)))
		return httpexpect.Default(t, startServerURL(t, cfg, seededRepos(t, false, seedActAsUser), nil, false)), idp
	}
	type call struct {
		auth  func(*svcIdP) map[string]string
		query string
		ip    string
	}
	anon := func(client string) func(*svcIdP) map[string]string {
		return func(idp *svcIdP) map[string]string { return actingFor(idp.clientToken(t, client)) }
	}
	forInt := func(client string) func(*svcIdP) map[string]string {
		return func(idp *svcIdP) map[string]string { return actingFor(idp.clientToken(t, client), asInt) }
	}
	own := func(idp *svcIdP) map[string]string {
		return actingFor(idp.token(t, jwt.MapClaims{"sub": auSubjectSub}))
	}
	noToken := func(*svcIdP) map[string]string { return map[string]string{} }

	cases := map[string]struct {
		first, second call
		shared        bool
	}{
		"one anonymous client from two addresses":   {call{anon("svc-a"), anonLookup, "192.0.2.1"}, call{anon("svc-a"), anonLookup, "192.0.2.2"}, true},
		"two anonymous clients":                     {call{anon("svc-a"), anonLookup, "192.0.2.1"}, call{anon("svc-b"), anonLookup, "192.0.2.1"}, false},
		"one integration client from two addresses": {call{forInt("svc-a"), intLookup, "192.0.2.1"}, call{forInt("svc-a"), intLookup, "192.0.2.2"}, true},
		"two integration clients":                   {call{forInt("svc-a"), intLookup, "192.0.2.1"}, call{forInt("svc-b"), intLookup, "192.0.2.1"}, false},
		"one user from two addresses":               {call{own, userLookup, "192.0.2.1"}, call{own, userLookup, "192.0.2.2"}, true},
		"no token from one address":                 {call{noToken, anonLookup, "192.0.2.1"}, call{noToken, anonLookup, "192.0.2.1"}, true},
		"no token from two addresses":               {call{noToken, anonLookup, "192.0.2.1"}, call{noToken, anonLookup, "192.0.2.2"}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, idp := start(t)
			do := func(c call) *httpexpect.Response {
				return e.POST("/api/graphql").WithHeaders(c.auth(idp)).WithHeader("X-Forwarded-For", c.ip).
					WithJSON(GraphQLRequest{Query: c.query}).Expect()
			}
			do(tc.first).Status(http.StatusOK)
			want := http.StatusOK
			if tc.shared {
				want = http.StatusTooManyRequests
			}
			do(tc.second).Status(want)
		})
	}
}
