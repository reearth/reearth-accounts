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
	viewer, ok := ctx.Value(lookupViewerKey{}).(string)
	if !ok {
		return nil
	}
	n := countLookupFields(oc.Operation.SelectionSet)
	if u.limiter.Allow(ctx, viewer, "graphql", n) {
		return nil
	}
	setResponseStatus(ctx, http.StatusTooManyRequests)
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

type lookupViewerKey struct{}

func withLookupViewer(ctx context.Context, viewer string) context.Context {
	return context.WithValue(ctx, lookupViewerKey{}, viewer)
}

type responseStatusKey struct{}

type responseStatus struct{ code int }

// withResponseStatus lets a GraphQL extension answer a status other than
// gqlgen's 200, such as 429 or 403, for an operation it refuses.
func withResponseStatus(ctx context.Context, res *echo.Response) context.Context {
	st := &responseStatus{}
	res.Before(func() {
		if st.code != 0 {
			res.Status = st.code
		}
	})
	return context.WithValue(ctx, responseStatusKey{}, st)
}

func setResponseStatus(ctx context.Context, code int) {
	if st, _ := ctx.Value(responseStatusKey{}).(*responseStatus); st != nil {
		st.code = code
	}
}
