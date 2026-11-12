package app

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// detailedOperationTracer creates a middleware that traces GraphQL operations with detailed attributes
func detailedOperationTracer() graphql.OperationMiddleware {
	tracer := otel.Tracer("reearth-accounts")

	return func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		opCtx := graphql.GetOperationContext(ctx)
		if opCtx == nil {
			return next(ctx)
		}

		operationName := opCtx.OperationName
		if operationName == "" {
			operationName = "anonymous"
		}

		spanName := "GraphQL " + string(opCtx.Operation.Operation) + " " + operationName

		// SpanKindInternal: the HTTP server span is already created by otelecho.
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("graphql.operation.name", operationName),
				attribute.String("graphql.operation.type", string(opCtx.Operation.Operation)),
				attribute.String("component", "graphql"),
			),
		)

		// The raw query and variables are not recorded: inline literals and
		// variable values may contain personal data.
		span.SetAttributes(
			attribute.StringSlice("graphql.operation.fields", topLevelFields(opCtx.Operation.SelectionSet, opCtx.Doc, nil)),
			attribute.Int("graphql.variables.count", len(opCtx.Variables)),
		)
		if u := adapter.User(ctx); u != nil {
			span.SetAttributes(semconv.EnduserID(u.ID().String()))
		}
		if caller := callerFrom(ctx); caller != "" {
			span.SetAttributes(attribute.String("accounts.caller", caller))
		}

		res := next(ctx)

		// span.End is deferred to the ResponseHandler so it covers the full
		// operation execution (res is invoked later by gqlgen).
		return func(ctx context.Context) *graphql.Response {
			defer span.End()
			response := res(ctx)

			if response != nil && len(response.Errors) > 0 {
				span.SetStatus(codes.Error, "GraphQL operation returned errors")
				// Only record error count; raw error messages may contain PII and are high-cardinality.
				span.SetAttributes(attribute.Int("graphql.errors.count", len(response.Errors)))
				// GraphQL errors include expected client-side failures (validation, auth, not found).
				// Log at Debug to avoid noisy WARN for non-actionable errors.
				applog.Debug(ctx, "graphql: operation completed with errors", applog.Operation(spanName), applog.Count(len(response.Errors)))
			} else {
				span.SetStatus(codes.Ok, "GraphQL operation completed successfully")
				applog.Debug(ctx, "graphql: operation completed", applog.Operation(spanName))
			}

			return response
		}
	}
}
