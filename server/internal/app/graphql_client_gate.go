package app

import (
	"context"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const errForbidden = "FORBIDDEN"

// clientFieldGate refuses a whole operation, before any resolver runs, when a
// client call selects a root field its subject kind is not admitted to. It
// reads the parsed operation, so persisted queries and every transport are
// covered.
type clientFieldGate struct {
	fields map[rootField]adapter.ClientAccess
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = clientFieldGate{}

func (clientFieldGate) ExtensionName() string { return "ClientFieldGate" }

func (clientFieldGate) Validate(graphql.ExecutableSchema) error { return nil }

func (g clientFieldGate) MutateOperationContext(ctx context.Context, oc *graphql.OperationContext) *gqlerror.Error {
	cc := adapter.ClientCallOf(ctx)
	if cc == nil || g.admits(oc.Operation.Operation, oc.Operation.SelectionSet, cc.Subject.Kind()) {
		return nil
	}
	setResponseStatus(ctx, http.StatusForbidden)
	err := gqlerror.Errorf("this operation is not open to client tokens")
	errcode.Set(err, errForbidden)
	return err
}

func (g clientFieldGate) admits(op ast.Operation, set ast.SelectionSet, k workspace.SubjectKind) bool {
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			if s.Name != "__typename" && !g.fields[rootField{op: op, name: s.Name}].Admits(k) {
				return false
			}
		case *ast.InlineFragment:
			if !g.admits(op, s.SelectionSet, k) {
				return false
			}
		case *ast.FragmentSpread:
			if s.Definition == nil || !g.admits(op, s.Definition.SelectionSet, k) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
