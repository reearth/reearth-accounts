package app

import (
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/vektah/gqlparser/v2/ast"
)

const publicDirective = "public"

// publicFields holds the lowercased names of the Query and Mutation fields
// marked @public in the schema. Only these may be called without a token.
var publicFields = publicFieldsOf(gql.NewExecutableSchema(gql.Config{}).Schema())

func publicFieldsOf(schema *ast.Schema) map[string]struct{} {
	fields := map[string]struct{}{}
	for _, root := range []*ast.Definition{schema.Query, schema.Mutation} {
		if root == nil {
			continue
		}
		for _, f := range root.Fields {
			if f.Directives.ForName(publicDirective) != nil {
				fields[strings.ToLower(f.Name)] = struct{}{}
			}
		}
	}
	return fields
}
