package e2e

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
)

const clientGateRefusal = "not open to client tokens"

func closedToClientsREST() [][2]string {
	ws := "/api/workspaces/" + svcWsOwned.String()
	return [][2]string{
		{http.MethodPost, "/api/users/signup"},
		{http.MethodPost, "/api/users/signup-oidc"},
		{http.MethodPost, "/api/users/verifications"},
		{http.MethodPost, "/api/users/verify"},
		{http.MethodPost, "/api/users/password-reset/start"},
		{http.MethodPost, "/api/users/password-reset"},
		{http.MethodGet, "/api/users/me"},
		{http.MethodPatch, "/api/users/me"},
		{http.MethodDelete, "/api/users/me"},
		{http.MethodDelete, "/api/users/me/auths/auth0|svc-owner"},
		{http.MethodGet, "/api/users/me/mfa"},
		{http.MethodPost, "/api/users/me/mfa"},
		{http.MethodDelete, "/api/users/me/mfa"},
		{http.MethodPost, "/api/users/me/mfa/recovery-code"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/users/search"},
		{http.MethodGet, "/api/users/by-alias"},
		{http.MethodGet, "/api/users/by-name-or-email"},
		{http.MethodGet, "/api/users/by-name-or-alias"},
		{http.MethodGet, "/api/users"},
		{http.MethodGet, "/api/users/all"},
		{http.MethodGet, "/api/users/" + svcOwnerUID.String()},
		{http.MethodPost, ws + "/scim/token"},
		{http.MethodDelete, ws + "/scim/token"},
		{http.MethodPut, ws + "/scim/config"},
		{http.MethodGet, ws + "/scim/config"},
	}
}

var closedToClientsGQL = []string{
	`mutation { signup(input: {name: "x", email: "x@e2e.com", password: "x"}) { user { id } } }`,
	`mutation { signupOIDC(input: {}) { user { id } } }`,
	`mutation { createVerification(input: {email: "svc-owner@e2e.com"}) }`,
	`mutation { verifyUser(input: {code: "x"}) { user { id } } }`,
	`mutation { startPasswordReset(input: {email: "svc-owner@e2e.com"}) }`,
	`mutation { passwordReset(input: {password: "x", token: "x"}) }`,
	`mutation { updateMe(input: {name: "x"}) { me { id } } }`,
	`mutation { removeMyAuth(input: {auth: "auth0"}) { me { id } } }`,
	`mutation { deleteMe(input: {userId: "x"}) { userId } }`,
	`mutation { enableMFA { enrollmentUrl } }`,
	`mutation { disableMFA }`,
	`mutation { regenerateMFARecoveryCode { recoveryCode } }`,
	`mutation { logout { id } }`,
	`{ me { id } }`,
	`{ mfaStatus { enrolled } }`,
	`{ searchUser(keyword: "svc") { id } }`,
	`{ userByNameOrEmail(nameOrEmail: "svc-owner") { id } }`,
	`{ findUserByAlias(alias: "x") { id } }`,
}

func TestClientGate_ClosedRoutesAndFields(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	tok := idp.clientToken(t, "svc-flow")
	callers := map[string]map[string]string{
		"acting for an integration": actingFor(tok, "integration:"+svcIntegration.String()),
		"with no header":            actingFor(tok),
	}
	for name, h := range callers {
		t.Run("REST "+name, func(t *testing.T) {
			for _, r := range closedToClientsREST() {
				e.Request(r[0], r[1]).WithHeaders(h).WithJSON(map[string]any{}).
					Expect().Status(http.StatusForbidden).Body().Contains(clientGateRefusal)
			}
		})
		t.Run("GraphQL "+name, func(t *testing.T) {
			for _, q := range closedToClientsGQL {
				refusedGQL(t, e, h, q, nil)
			}
		})
	}
}

func TestClientGate_UserTokenIsUntouched(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	h := actingFor(idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"}))
	serviceGQL(e, h, `{ me { id } findByUser(userId: "`+svcOwnerUID.String()+`") { id } }`, nil).
		Value("data").Object().Value("me").Object().HasValue("id", svcOwnerUID.String())
	for _, r := range closedToClientsREST() {
		body := e.Request(r[0], r[1]).WithHeaders(h).WithJSON(map[string]any{}).Expect().Body().Raw()
		assert.NotContains(t, body, clientGateRefusal, "%s %s", r[0], r[1])
	}
	for _, q := range closedToClientsGQL {
		body := e.POST("/api/graphql").WithHeaders(h).WithJSON(GraphQLRequest{Query: q}).Expect().Body().Raw()
		assert.NotContains(t, body, clientGateRefusal, q)
	}
}

func TestClientGate_GraphQLDocumentShapes(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	flow := actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcIntegration.String())
	byUser := `findByUser(userId: "` + svcOwnerUID.String() + `") { id }`

	for name, q := range map[string]string{
		"an allowed and a closed field":            `{ ` + byUser + ` me { id } }`,
		"a closed field behind an alias":           `{ ws: ` + byUser + ` findByUser: searchUser(keyword: "svc") { id } }`,
		"a closed field in a spread":               `query { ` + byUser + ` ...F } fragment F on Query { searchUser(keyword: "svc") { id } }`,
		"a closed field in a nested spread":        `query { ...A } fragment A on Query { ...B } fragment B on Query { me { id } }`,
		"a closed field in a fragment":             `{ ` + byUser + ` ... on Query { me { id } } }`,
		"a closed mutation":                        `mutation { logout { id } }`,
		"a closed field under @skip(if: true)":     `{ ` + byUser + ` me @skip(if: true) { id } }`,
		"a closed field under @include(if: false)": `{ ` + byUser + ` me @include(if: false) { id } }`,
		"an introspection field":                   `{ __schema { queryType { name } } }`,
	} {
		t.Run(name+" is refused as a whole", func(t *testing.T) {
			refusedGQL(t, e, flow, q, nil)
		})
	}

	t.Run("__typename passes", func(t *testing.T) {
		serviceGQL(e, flow, `{ __typename `+byUser+` }`, nil).Value("data").Object().ContainsKey("findByUser")
	})

	t.Run("the gate checks the selected operation", func(t *testing.T) {
		doc := `query Open { ` + byUser + ` } query Closed { me { id } }`
		e.POST("/api/graphql").WithHeaders(flow).WithJSON(GraphQLRequest{Query: doc, OperationName: "Open"}).
			Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().ContainsKey("findByUser")
		e.POST("/api/graphql").WithHeaders(flow).WithJSON(GraphQLRequest{Query: doc, OperationName: "Closed"}).
			Expect().Status(http.StatusForbidden)
	})
}

func TestClientGate_ActAsOnRoutesWithoutAuthMiddleware(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	userTok := idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"})
	asInt := "integration:" + svcIntegration.String()
	for name, h := range map[string]map[string]string{
		"a user token": actingFor(userTok, asInt),
		"no token":     {app.ActAsHeader: asInt},
	} {
		t.Run(name, func(t *testing.T) {
			for _, p := range []string{"/api/users/verify", "/api/users/password-reset/start", "/api/users/password-reset"} {
				e.POST(p).WithHeaders(h).WithJSON(map[string]any{}).Expect().Status(http.StatusUnauthorized)
			}
			e.GET("/api/auth/config").WithHeaders(h).Expect().Status(http.StatusUnauthorized)
		})
	}
}

func TestClientGate_UnroutedAPIPathsAre403(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	tok := idp.clientToken(t, "svc-flow")
	callers := map[string]map[string]string{
		"acting for an integration": actingFor(tok, "integration:"+svcIntegration.String()),
		"with no header":            actingFor(tok),
	}
	for _, r := range [][2]string{
		{http.MethodGet, "/api"},
		{http.MethodGet, "/api/graphql"},
		{http.MethodPost, "/api/graphql/x"},
		{http.MethodOptions, "/api/graphql"},
	} {
		for name, h := range callers {
			t.Run(r[0]+" "+r[1]+" "+name, func(t *testing.T) {
				e.Request(r[0], r[1]).WithHeaders(h).Expect().
					Status(http.StatusForbidden).Body().Contains(clientGateRefusal)
			})
		}
		t.Run(r[0]+" "+r[1]+" with no token", func(t *testing.T) {
			e.Request(r[0], r[1]).Expect().Status(http.StatusNotFound)
		})
	}
}

func TestClientGate_AuthConfigIsOpenToIntegrations(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	flow := actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcIntegration.String())
	e.GET("/api/auth/config").WithHeaders(flow).Expect().Status(http.StatusOK)
	publicGraphQL(e, flow).Status(http.StatusOK).JSON().Object().Value("data").Object().ContainsKey("authConfig")
	refusedGQL(t, e, flow, `query($u: [ID!]!) { findUsersByIDsWithPagination(ids: $u, pagination: {page: 1, size: 1}) { totalCount } }`,
		map[string]any{"u": []string{svcOwnerUID.String()}})
}
