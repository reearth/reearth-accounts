package app

import (
	"context"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const errRateLimited = "RATE_LIMITED"

// userLookupFields are the root fields that return users chosen by the caller.
var userLookupFields = map[string]struct{}{
	"node":                         {},
	"nodes":                        {},
	"user":                         {},
	"searchUser":                   {},
	"findUserByAlias":              {},
	"findUsersByIDs":               {},
	"findUsersByIDsWithPagination": {},
	"userByNameOrAlias":            {},
	"userByNameOrEmail":            {},
}

// userLookupLimit charges one lookup per user-lookup root field, so aliasing
// a field many times in one operation costs as much as many requests.
type userLookupLimit struct{ limiter *ratelimit.Limiter }

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = userLookupLimit{}

func (userLookupLimit) ExtensionName() string { return "UserLookupLimit" }

func (userLookupLimit) Validate(graphql.ExecutableSchema) error { return nil }

func (u userLookupLimit) MutateOperationContext(ctx context.Context, oc *graphql.OperationContext) *gqlerror.Error {
	st, _ := ctx.Value(lookupStateKey{}).(*lookupState)
	if st == nil {
		return nil
	}
	n := countLookupFields(oc.Operation.SelectionSet)
	if u.limiter.Allow(ctx, st.viewer, "graphql", n) {
		return nil
	}
	st.limited = true
	err := gqlerror.Errorf("too many user lookups; retry later")
	errcode.Set(err, errRateLimited)
	return err
}

func countLookupFields(set ast.SelectionSet) int {
	n := 0
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			if _, ok := userLookupFields[s.Name]; ok {
				n++
			}
		case *ast.InlineFragment:
			n += countLookupFields(s.SelectionSet)
		case *ast.FragmentSpread:
			n += countLookupFields(s.Definition.SelectionSet)
		}
	}
	return n
}

type lookupStateKey struct{}

// lookupState carries the viewer into the extension and its verdict back out,
// so the handler can answer 429 instead of gqlgen's 200.
type lookupState struct {
	viewer  string
	limited bool
}

func withLookupState(ctx context.Context, viewer string, res *echo.Response) context.Context {
	st := &lookupState{viewer: viewer}
	res.Before(func() {
		if st.limited {
			res.Status = http.StatusTooManyRequests
		}
	})
	return context.WithValue(ctx, lookupStateKey{}, st)
}
