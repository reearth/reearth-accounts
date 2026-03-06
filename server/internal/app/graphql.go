package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/storage"
	"github.com/reearth/reearthx/log"

	"github.com/labstack/echo/v4"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	enableDataLoaders = true
	maxUploadSize     = 10 * 1024 * 1024 * 1024 // 10GB
	maxMemorySize     = 100 * 1024 * 1024       // 100MB
)

func GraphqlAPI(conf *Config, dev bool) echo.HandlerFunc {
	str, err := storage.NewGCPStorage(&storage.Config{
		IsLocal:          conf.StorageIsLocal,
		BucketName:       conf.StorageBucketName,
		EmulatorEnabled:  conf.StorageEmulatorEnabled,
		EmulatorEndpoint: conf.StorageEmulatorEndpoint,
	})
	if err != nil {
		log.Fatal("failed to initialize storage: " + err.Error())
	}

	schema := gql.NewExecutableSchema(gql.Config{
		Resolvers: gql.NewResolver(str),
	})

	srv := handler.New(schema)
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxUploadSize: maxUploadSize,
		MaxMemory:     maxMemorySize,
	})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(otelgqlgen.Middleware())

	if conf.GraphQL.ComplexityLimit > 0 {
		srv.Use(extension.FixedComplexityLimit(conf.GraphQL.ComplexityLimit))
	}

	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](30),
	})

	// Mutation field-level transfer size logging
	srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
		rc := graphql.GetOperationContext(ctx)
		fc := graphql.GetFieldContext(ctx)

		res, err = next(ctx)

		// Only log for mutation fields (top-level mutation fields)
		if rc.Operation != nil && rc.Operation.Operation == ast.Mutation && fc.Object == "Mutation" {
			fieldSize := 0
			if res != nil {
				if resJSON, jsonErr := json.Marshal(res); jsonErr == nil {
					fieldSize = len(resJSON)
				}
			}

			log.Infof("Mutation field: %s response_size=%d bytes", fc.Field.Name, fieldSize)
		}

		// Dev mode field logging
		if dev {
			fmt.Printf("Resolving %s.%s\n", fc.Object, fc.Field.Name)
			if err != nil {
				fmt.Printf("Error in %s.%s: %v\n", fc.Object, fc.Field.Name, err)
			}
		}

		return res, err
	})

	// Mutation total transfer size logging (always enabled)
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		rc := graphql.GetOperationContext(ctx)
		resp := next(ctx)

		// Only log for mutations
		if rc.Operation != nil && rc.Operation.Operation == ast.Mutation {
			operationName := rc.OperationName
			if operationName == "" {
				operationName = "anonymous"
			}

			// Calculate request size
			requestSize := len(rc.RawQuery)
			if rc.Variables != nil {
				if varsJSON, err := json.Marshal(rc.Variables); err == nil {
					requestSize += len(varsJSON)
				}
			}

			// Calculate response size
			responseSize := 0
			if resp != nil && resp.Data != nil {
				responseSize = len(resp.Data)
			}

			log.Infof("Mutation transfer total: operation=%s request_size=%d bytes response_size=%d bytes total=%d bytes",
				operationName, requestSize, responseSize, requestSize+responseSize)
		}

		// Dev mode error logging
		if dev && resp != nil && len(resp.Errors) > 0 {
			fmt.Printf("\n⚠️ GraphQL Errors:\n")
			for _, e := range resp.Errors {
				fmt.Printf("Message: %s\nPath: %v\nExtensions: %+v\n\n", e.Message, e.Path, e.Extensions)
			}
		}

		return resp
	})

	if dev {
		srv.Use(extension.Introspection{})
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			rc := graphql.GetOperationContext(ctx)
			log.Printf("GraphQL Request:\nQuery:\n%s\nVariables: %+v\n", rc.RawQuery, rc.Variables)
			return next(ctx)
		})
	}

	return func(c echo.Context) error {
		req := c.Request()
		ctx := req.Context()

		srv.SetErrorPresenter(gqlErrorPresenter(dev))

		usecases := adapter.Usecases(ctx)
		ctx = gql.AttachUsecases(ctx, usecases, str, enableDataLoaders)
		ctx = adapter.AttachConfig(ctx, conf)
		c.SetRequest(req.WithContext(ctx))

		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

func gqlErrorPresenter(dev bool) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, e error) *gqlerror.Error {
		if dev {
			return gqlerror.ErrorPathf(graphql.GetFieldContext(ctx).Path(), "%s", e.Error())
		}

		return graphql.DefaultErrorPresenter(ctx, e)
	}
}
