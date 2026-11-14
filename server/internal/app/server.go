package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/labstack/echo/v4"
	infraCerbos "github.com/reearth/reearth-accounts/server/internal/infrastructure/cerbos"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/migration"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	pgmigration "github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearthx/log"

	otelapp "github.com/reearth/reearth-accounts/server/internal/app/otel"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
	"golang.org/x/net/http2"
)

func Start(debug bool) {

	ctx := context.Background()

	// Load config
	conf, cerr := ReadConfig(debug)
	if cerr != nil {
		applog.Fatal(ctx, "failed to read config", applog.Err(cerr))
	}
	applog.Info(ctx, "config", applog.Config(conf.Print()))
	log.GCPProject = gcpProject(ctx, conf)

	// Init OpenTelemetry tracer
	if conf.OtelEnabled {
		tp, terr := otelapp.InitTracer(ctx, &otelapp.Config{
			Enabled:            conf.OtelEnabled,
			Endpoint:           conf.OtelEndpoint,
			ExporterType:       otelapp.ExporterType(conf.OtelExporterType),
			Insecure:           conf.OtelInsecure,
			BatchTimeout:       conf.OtelBatchTimeout,
			MaxExportBatchSize: conf.OtelMaxExportBatchSize,
			MaxQueueSize:       conf.OtelMaxQueueSize,
			SamplingRatio:      conf.OtelSamplingRatio,
			ServiceName:        otelapp.OtelAccountsServiceName,
		})
		if terr != nil {
			applog.Warn(ctx, "failed to init tracer", applog.Err(terr))
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if serr := tp.Shutdown(shutdownCtx); serr != nil {
					applog.Error(ctx, "failed to shutdown tracer", applog.Err(serr))
				}
			}()
		}
	}

	var repos *repo.Container
	var gateways *gateway.Container

	if conf.ResolveDBDriver() == "postgres" {
		// 30s startup budget covers connect+ping+migrate.
		pgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		pool, perr := postgres.NewPool(pgCtx, conf.DB)
		if perr != nil {
			applog.Fatal(ctx, "postgres pool init error", applog.Err(perr))
		}
		if perr := pool.Ping(pgCtx); perr != nil {
			applog.Fatal(ctx, "postgres ping error", applog.Err(perr))
		}
		if merr := pgmigration.Migrate(pgCtx, pool); merr != nil {
			applog.Fatal(ctx, "postgres migration error", applog.Err(merr))
		}
		repos, gateways = initPostgresReposAndGateways(ctx, pool, conf)
	} else {
		// Init MongoDB client with optional command monitoring
		var debugMonitor *event.CommandMonitor
		if debug || os.Getenv("REEARTH_ACCOUNTS_DEV") == "true" {
			debugMonitor = &event.CommandMonitor{
				Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
					applog.Error(ctx, "MongoDB Command Failed", applog.Command(evt.CommandName), applog.DurationMS(evt.Duration), applog.ErrMessage(evt.Failure))
				},
				Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
					// Only log slow queries or critical operations
					if evt.Duration > time.Millisecond*100 ||
						evt.CommandName == "createIndexes" ||
						evt.CommandName == "dropIndexes" ||
						evt.CommandName == "drop" {
						applog.Debug(ctx, "MongoDB Command", applog.Command(evt.CommandName), applog.DurationMS(evt.Duration))
					}
				},
			}
		}

		var otelMonitor *event.CommandMonitor
		if conf.OtelEnabled {
			otelMonitor = otelmongo.NewMonitor()
		}

		client, err := mongo.Connect(
			ctx,
			options.Client().
				ApplyURI(conf.DB).
				SetConnectTimeout(time.Second*10).
				SetMonitor(chainMongoMonitors(debugMonitor, otelMonitor)))
		if err != nil {
			applog.Fatal(ctx, "repo initialization error", applog.Err(err))
		}

		// Init repositories
		repos, gateways = initReposAndGateways(ctx, client, conf)

		// Check if migration mode (mongo-only data migrations).
		// Once the permission check migration is complete, it will be deleted.
		if os.Getenv("RUN_MIGRATION") == "true" {
			clientx := mongox.NewClient(conf.DBName, client)
			db := clientx.Database()

			lock, lockErr := mongorepo.NewLock(db.Collection("locks"))
			if lockErr != nil {
				applog.Fatal(ctx, "failed to create lock", applog.Err(lockErr))
			}

			if migrationErr := runMigration(ctx, repos); migrationErr != nil {
				applog.Fatal(ctx, "failed to run data migration", applog.Err(migrationErr))
			}

			if migrationErr := migration.Do(ctx, clientx, mongorepo.NewConfig(db.Collection("config"), lock)); migrationErr != nil {
				applog.Fatal(ctx, "failed to run migration", applog.Err(migrationErr))
			}
			return
		}
	}

	// Cerbos
	cerbosClient, err := infraCerbos.NewClient(conf.CerbosHost, conf.CerbosUseSSL)
	if err != nil {
		applog.Fatal(ctx, "Failed to create cerbos client", applog.Err(err))
	}
	cerbosAdapter := infraCerbos.NewCerbosAdapter(cerbosClient)

	// Start web server
	NewServer(ctx, &ServerConfig{
		Config:        conf,
		Debug:         debug,
		Repos:         repos,
		Gateways:      gateways,
		CerbosAdapter: cerbosAdapter,
	}).Run(ctx)
}

type WebServer struct {
	address   string
	appServer *echo.Echo
}

type ServerConfig struct {
	Config        *Config
	Debug         bool
	Repos         *repo.Container
	Gateways      *gateway.Container
	CerbosAdapter gateway.CerbosGateway
}

func NewServer(ctx context.Context, cfg *ServerConfig) *WebServer {
	port := cfg.Config.Port
	host := cfg.Config.Host

	if port == "" {
		port = "8080"
	}

	if host == "" {
		if cfg.Debug {
			host = "localhost"
		} else {
			host = "0.0.0.0"
		}
	}

	address := host + ":" + port

	w := &WebServer{
		address: address,
	}

	w.appServer = initEcho(ctx, cfg)
	return w
}

func (w *WebServer) Run(ctx context.Context) {
	defer applog.Info(ctx, "Server shutdown")

	msg := "server started"
	if w.appServer.Debug {
		msg = "server started with debug mode"
	}
	applog.Info(ctx, msg, applog.Address(w.address))

	go func() {
		if err := w.appServer.StartH2CServer(w.address, &http2.Server{}); err != nil && !errors.Is(err, http.ErrServerClosed) {
			applog.Fatal(ctx, "server stopped", applog.Err(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.Shutdown(shutdownCtx); err != nil {
		applog.Error(ctx, "server shutdown", applog.Err(err))
	}
}

func (w *WebServer) Serve(l net.Listener) error {
	return w.appServer.Server.Serve(l)
}

func (w *WebServer) Shutdown(ctx context.Context) error {
	return w.appServer.Shutdown(ctx)
}

func chainMongoMonitors(monitors ...*event.CommandMonitor) *event.CommandMonitor {
	active := monitors[:0]
	for _, m := range monitors {
		if m != nil {
			active = append(active, m)
		}
	}
	if len(active) == 0 {
		return nil
	}
	if len(active) == 1 {
		return active[0]
	}
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			for _, m := range active {
				if m.Started != nil {
					m.Started(ctx, evt)
				}
			}
		},
		Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
			for _, m := range active {
				if m.Succeeded != nil {
					m.Succeeded(ctx, evt)
				}
			}
		},
		Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
			for _, m := range active {
				if m.Failed != nil {
					m.Failed(ctx, evt)
				}
			}
		},
	}
}

// gcpProject names the project that log entries link their trace to. Cloud
// Run does not set GOOGLE_CLOUD_PROJECT, so on Cloud Run it asks the metadata
// server.
func gcpProject(ctx context.Context, conf *Config) string {
	if conf.GCPProject != "" || os.Getenv("K_SERVICE") == "" {
		return conf.GCPProject
	}
	id, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		applog.Warn(ctx, "log entries will not link to traces: GCP project unknown", applog.Err(err))
		return ""
	}
	return id
}
