package app

import (
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/vektah/gqlparser/v2/ast"
)

const clientDirective = "client"

var clientSubjects = map[string]adapter.ClientAccess{
	"INTEGRATION": adapter.IntegrationClients,
	"USER":        adapter.UserClients,
}

type rootField struct {
	op   ast.Operation
	name string
}

// clientFields holds, per root operation and field, the client access the
// schema grants: the @client kinds, plus anonymous clients for @public Query
// fields. A @public Mutation grants nothing, so signup and its siblings stay
// closed to every client token.
var clientFields = clientFieldsOf(gql.NewExecutableSchema(gql.Config{}).Schema())

func clientFieldsOf(schema *ast.Schema) map[rootField]adapter.ClientAccess {
	fields := map[rootField]adapter.ClientAccess{}
	for op, root := range map[ast.Operation]*ast.Definition{ast.Query: schema.Query, ast.Mutation: schema.Mutation} {
		if root == nil {
			continue
		}
		for _, f := range root.Fields {
			var a adapter.ClientAccess
			if d := f.Directives.ForName(clientDirective); d != nil {
				for _, v := range d.Arguments.ForName("as").Value.Children {
					a |= clientSubjects[v.Value.Raw]
				}
			}
			if op == ast.Query && f.Directives.ForName(publicDirective) != nil {
				a |= adapter.AnonymousClients
			}
			if a != 0 {
				fields[rootField{op: op, name: f.Name}] = a
			}
		}
	}
	return fields
}
