package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

type rootField struct {
	op     string
	name   string
	public bool
}

func schemaRootFields(t *testing.T) []rootField {
	t.Helper()
	s := gql.NewExecutableSchema(gql.Config{}).Schema()
	var out []rootField
	for op, def := range map[string]*ast.Definition{"query": s.Query, "mutation": s.Mutation} {
		for _, f := range def.Fields {
			if f.Name == "__schema" || f.Name == "__type" || f.Name == "__typename" {
				continue
			}
			out = append(out, rootField{op: op, name: f.Name, public: f.Directives.ForName("public") != nil})
		}
	}
	return out
}

// ADR-0003 decision 1: every field not marked @public needs a token.
func TestGQLPrivateFieldsRejectAnonymous(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, baseSeederAuthBypass)

	checked := 0
	for _, f := range schemaRootFields(t) {
		if f.public {
			continue
		}
		checked++
		t.Run(f.op+"."+f.name, func(t *testing.T) {
			e.POST("/api/graphql").
				WithJSON(GraphQLRequest{Query: f.op + " { " + f.name + " }"}).
				Expect().Status(http.StatusUnauthorized)
		})
	}
	assert.Greater(t, checked, 20)
}

// A valid anonymous call for each @public field. A new @public field fails
// this test until it gets an entry here.
var publicFieldCalls = map[string]GraphQLRequest{
	"authConfig":                   {Query: `{ authConfig { auth0Domain } }`},
	"findByID":                     {Query: `query($id: ID!) { findByID(id: $id) { id } }`, Variables: map[string]any{"id": wId.String()}},
	"findByIDs":                    {Query: `query($ids: [ID!]!) { findByIDs(ids: $ids) { id } }`, Variables: map[string]any{"ids": []string{wId.String()}}},
	"findByAlias":                  {Query: `{ findByAlias(alias: "nope") { id } }`},
	"findUsersByIDsWithPagination": {Query: `query($ids: [ID!]!) { findUsersByIDsWithPagination(ids: $ids, pagination: {page: 1, size: 10}) { totalCount } }`, Variables: map[string]any{"ids": []string{uId.String()}}},
	"publicWorkspace":              {Query: `{ publicWorkspace(alias: "nope") { id } }`},
	"createVerification":           {Query: `mutation { createVerification(input: {email: "nobody@example.com"}) }`},
	"startPasswordReset":           {Query: `mutation { startPasswordReset(input: {email: "nobody@example.com"}) }`},
	"signup":                       {Query: `mutation { signup(input: {name: "anon", email: "anon@example.com", password: "Passw0rd!Passw0rd"}) { user { id } } }`},
	"signupOIDC":                   {Query: `mutation { signupOIDC(input: {name: "anon2", email: "anon2@example.com", sub: "auth0|anon2"}) { user { id } } }`},
}

func TestGQLPublicFieldsAllowAnonymous(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, baseSeederAuthBypass)

	var public []string
	for _, f := range schemaRootFields(t) {
		if f.public {
			public = append(public, f.name)
		}
	}
	assert.ElementsMatch(t, public, keys(publicFieldCalls))

	for name, req := range publicFieldCalls {
		t.Run(name, func(t *testing.T) {
			e.POST("/api/graphql").
				WithJSON(req).
				Expect().Status(http.StatusOK).
				JSON().Object().ContainsKey("data")
		})
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
