package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearthx/log"
	"github.com/vektah/gqlparser/v2/ast"
)

const (
	internalServiceHeader = "X-Internal-Service"
	maxCallerLabelLen     = 64
	maxLoggedFields       = 32
	m2mSubSuffix          = "@clients"
)

// requestRecord collects what inner handlers learn about a request, so the
// request logger can write one entry when the request ends.
type requestRecord struct {
	mu            sync.Mutex
	operation     string
	operationType string
	fields        []string
}

type requestRecordKey struct{}

// RequestLogger writes one entry per request with its route or GraphQL
// operation, top-level fields, status, latency, viewer and calling service
// (ADR-0003 decision 9). It never logs the query string, variables, bodies,
// IP address or user agent. It also attaches a request-scoped logger carrying
// request_id to the request context.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()

			reqID := log.GetReqestID(c.Response(), req)
			if reqID == "" {
				reqID = newRequestID()
				c.Response().Header().Set(echo.HeaderXRequestID, reqID)
			}

			rec := &requestRecord{}
			ctx := applog.With(req.Context(), applog.RequestID(reqID))
			ctx = context.WithValue(ctx, requestRecordKey{}, rec)
			c.SetRequest(req.WithContext(ctx))

			if err := next(c); err != nil {
				c.Error(err)
			}

			// Inner middleware replaces the request, so read the final context:
			// it carries the authenticated viewer.
			finalCtx := c.Request().Context()
			applog.Info(ctx, "request", requestFields(c, finalCtx, rec, time.Since(start))...)
			return nil
		}
	}
}

func requestFields(c echo.Context, ctx context.Context, rec *requestRecord, latency time.Duration) []applog.Field {
	route := c.Path()
	if route == "" {
		route = "unmatched"
	}
	fields := []applog.Field{
		applog.Method(c.Request().Method),
		applog.Route(route),
		applog.Status(c.Response().Status),
		applog.LatencyMS(latency),
	}

	rec.mu.Lock()
	if rec.operation != "" {
		fields = append(fields, applog.Operation(rec.operation), applog.OperationType(rec.operationType))
	}
	if len(rec.fields) > 0 {
		fields = append(fields, applog.Fields(rec.fields))
	}
	rec.mu.Unlock()

	if u := adapter.User(ctx); u != nil {
		fields = append(fields, applog.ViewerID(u.ID().String()))
	}
	if caller := callerLabel(c.Request().Header.Get(internalServiceHeader)); caller != "" {
		fields = append(fields, applog.Caller(caller))
	}
	if ai := adapter.GetAuthInfo(ctx); ai != nil && strings.HasSuffix(ai.Sub, m2mSubSuffix) {
		fields = append(fields, applog.CallerClientID(strings.TrimSuffix(ai.Sub, m2mSubSuffix)))
	}
	return fields
}

// callerLabel keeps the caller-chosen X-Internal-Service value short and
// plain, since it is a label and proves nothing about the caller.
func callerLabel(v string) string {
	if len(v) > maxCallerLabelLen {
		v = v[:maxCallerLabelLen]
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return -1
	}, v)
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// recordGraphQLOperation is a gqlgen operation middleware that tells the
// request logger which operation and top-level fields a request ran.
func recordGraphQLOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	rec, _ := ctx.Value(requestRecordKey{}).(*requestRecord)
	opCtx := graphql.GetOperationContext(ctx)
	if rec != nil && opCtx != nil && opCtx.Operation != nil {
		name := opCtx.OperationName
		if name == "" {
			name = "anonymous"
		}
		fields := topLevelFields(opCtx.Operation.SelectionSet, opCtx.Doc, nil)
		rec.mu.Lock()
		rec.operation = name
		rec.operationType = string(opCtx.Operation.Operation)
		rec.fields = fields
		rec.mu.Unlock()
	}
	return next(ctx)
}

// topLevelFields lists the schema field names an operation selects at its
// root, following fragments. Aliases are not logged; they are caller-chosen.
func topLevelFields(set ast.SelectionSet, doc *ast.QueryDocument, out []string) []string {
	for _, s := range set {
		if len(out) >= maxLoggedFields {
			return out
		}
		switch sel := s.(type) {
		case *ast.Field:
			if !slices.Contains(out, sel.Name) {
				out = append(out, sel.Name)
			}
		case *ast.InlineFragment:
			out = topLevelFields(sel.SelectionSet, doc, out)
		case *ast.FragmentSpread:
			if sel.Definition != nil {
				out = topLevelFields(sel.Definition.SelectionSet, doc, out)
			} else if doc != nil {
				if def := doc.Fragments.ForName(sel.Name); def != nil {
					out = topLevelFields(def.SelectionSet, doc, out)
				}
			}
		}
	}
	return out
}
