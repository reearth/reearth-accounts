package app

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/storage"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/applog"

	"github.com/labstack/echo/v4"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	enableDataLoaders = true
	maxUploadSize     = 10 * 1024 * 1024 * 1024 // 10GB
	maxMemorySize     = 100 * 1024 * 1024       // 100MB
)

func newPhotoStorage(conf *Config) gateway.Storage {
	str, err := storage.NewGCPStorage(&storage.Config{
		IsLocal:          conf.StorageIsLocal,
		BucketName:       conf.StorageBucketName,
		EmulatorEnabled:  conf.StorageEmulatorEnabled,
		EmulatorEndpoint: conf.StorageEmulatorEndpoint,
	})
	if err != nil {
		applog.Fatal(context.Background(), "failed to initialize storage", applog.Err(err))
	}
	return str
}

func GraphqlAPI(conf *Config, photos adapter.PhotoSigner, loginTime adapter.LoginTimeSource, lookups *ratelimit.Limiter, dev bool) echo.HandlerFunc {
	schema := gql.NewExecutableSchema(gql.Config{
		Resolvers:  gql.NewResolver(photos, loginTime),
		Directives: gql.NewDirectives(),
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

	// tracing with detailed operation tracking
	srv.AroundOperations(detailedOperationTracer())
	srv.AroundOperations(recordGraphQLOperation)

	if conf.GraphQL.ComplexityLimit > 0 {
		srv.Use(extension.FixedComplexityLimit(conf.GraphQL.ComplexityLimit))
	}
	if conf.GraphQL.DepthLimit > 0 {
		srv.Use(depthLimit{max: conf.GraphQL.DepthLimit})
	}
	srv.Use(userLookupLimit{limiter: lookups})

	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](30),
	})

	srv.SetErrorPresenter(gqlErrorPresenter(dev))

	if dev {
		srv.Use(extension.Introspection{})
		srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
			res, err = next(ctx)
			if err != nil {
				fc := graphql.GetFieldContext(ctx)
				applog.Debug(ctx, "graphql: field failed", applog.Operation(fc.Object+"."+fc.Field.Name), applog.Err(err))
			}
			return res, err
		})
	}

	return func(c echo.Context) error {
		req := c.Request()
		ctx := req.Context()

		tracer := otel.Tracer("reearth-accounts")
		// SpanKindInternal: the HTTP server span is already created by otelecho.
		ctx, span := tracer.Start(ctx, "GraphQL Handler",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("component", "graphql"),
				attribute.String("http.method", req.Method),
				attribute.String("http.url", req.URL.Path),
				attribute.String("handler", "graphql"),
			),
		)
		defer span.End()

		usecases := adapter.Usecases(ctx)
		ctx = gql.AttachUsecases(ctx, usecases, photos, enableDataLoaders)
		ctx = adapter.AttachConfig(ctx, conf)
		ctx = withLookupState(ctx, ratelimit.ViewerKey(adapter.User(ctx), c.RealIP()), c.Response())
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
