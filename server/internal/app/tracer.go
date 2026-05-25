package app

import (
	"context"
	"sort"

	"github.com/99designs/gqlgen/graphql"
	"github.com/reearth/reearthx/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// maxVariableNamesRecorded caps the number of GraphQL variable names recorded
// per span so a malicious or noisy query cannot blow up trace payload size.
const maxVariableNamesRecorded = 16

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

		// Raw query is intentionally not recorded: inline literals may contain PII.
		// Variable values are also not recorded. Variable names are recorded as a
		// single bounded list to avoid unbounded high-cardinality attribute keys
		// (GraphQL variable names are user-controlled).
		if n := len(opCtx.Variables); n > 0 {
			span.SetAttributes(attribute.Int("graphql.variables.count", n))
			// Cap collection up front: variable names are user-controlled, so
			// allocating a slice of size n and sorting all of it would let a
			// caller amplify CPU/allocations by sending many variables.
			capLen := n
			if capLen > maxVariableNamesRecorded {
				capLen = maxVariableNamesRecorded
			}
			names := make([]string, 0, capLen)
			for key := range opCtx.Variables {
				if len(names) >= maxVariableNamesRecorded {
					break
				}
				names = append(names, key)
			}
			sort.Strings(names)
			span.SetAttributes(attribute.StringSlice("graphql.variables.names", names))
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
				log.Debugfc(ctx, "graphql: operation '%s' completed with %d errors", spanName, len(response.Errors))
			} else {
				span.SetStatus(codes.Ok, "GraphQL operation completed successfully")
				log.Debugfc(ctx, "graphql: operation '%s' completed successfully", spanName)
			}

			return response
		}
	}
}

// responseTracer creates a middleware that traces response handling
func responseTracer() graphql.ResponseMiddleware {
	return func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		span := trace.SpanFromContext(ctx)

		response := next(ctx)

		if response != nil {
			if response.Extensions != nil {
				if complexity, ok := response.Extensions["complexity"].(int); ok {
					span.SetAttributes(attribute.Int("graphql.response.complexity", complexity))
				}
			}
		}

		return response
	}
}
