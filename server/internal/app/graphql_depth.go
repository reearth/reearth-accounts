package app

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const errDepthLimitExceeded = "DEPTH_LIMIT_EXCEEDED"

// depthLimit rejects operations whose field nesting exceeds max. A root field
// has depth 1; fragments add no depth of their own. Introspection is exempt:
// it is enabled only in development, and client tooling sends introspection
// queries about 12 levels deep.
type depthLimit struct{ max int }

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = depthLimit{}

func (depthLimit) ExtensionName() string { return "DepthLimit" }

func (depthLimit) Validate(graphql.ExecutableSchema) error { return nil }

func (d depthLimit) MutateOperationContext(_ context.Context, oc *graphql.OperationContext) *gqlerror.Error {
	if depth := selectionDepth(oc.Operation.SelectionSet); depth > d.max {
		err := gqlerror.Errorf("operation has depth %d, which exceeds the limit of %d", depth, d.max)
		errcode.Set(err, errDepthLimitExceeded)
		return err
	}
	return nil
}

func selectionDepth(set ast.SelectionSet) int {
	deepest := 0
	for _, sel := range set {
		var d int
		switch s := sel.(type) {
		case *ast.Field:
			if s.Name == "__schema" || s.Name == "__type" {
				continue
			}
			d = 1 + selectionDepth(s.SelectionSet)
		case *ast.InlineFragment:
			d = selectionDepth(s.SelectionSet)
		case *ast.FragmentSpread:
			d = selectionDepth(s.Definition.SelectionSet)
		}
		deepest = max(deepest, d)
	}
	return deepest
}
