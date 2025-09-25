package app

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/reearth/reearth-accounts/internal/adapter"
	"github.com/reearth/reearth-accounts/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/internal/infrastructure/storage"
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

	if dev {
		srv.Use(extension.Introspection{})
	}

	return func(c echo.Context) error {
		req := c.Request()
		ctx := req.Context()

		srv.SetErrorPresenter(gqlErrorPresenter(dev))

		usecases := adapter.Usecases(ctx)
		ctx = gql.AttachUsecases(ctx, usecases, enableDataLoaders)
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
