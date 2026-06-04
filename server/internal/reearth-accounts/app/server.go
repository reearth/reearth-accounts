package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/labstack/echo/v4"
	infraCerbos "github.com/reearth/reearth-accounts/server/internal/infrastructure/cerbos"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/migration"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"

	otelapp "github.com/reearth/reearth-accounts/server/internal/reearth-accounts/app/otel"
	"github.com/reearth/reearthx/log"
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
		log.Fatal(cerr)
	}
	log.Infof("config: %s", conf.Print())

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
			log.Warnfc(ctx, "failed to init tracer: %v", terr)
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if serr := tp.Shutdown(shutdownCtx); serr != nil {
					log.Errorfc(ctx, "failed to shutdown tracer: %s", serr.Error())
				}
			}()
		}
	}

	// Init MongoDB client with optional command monitoring
	var debugMonitor *event.CommandMonitor
	if debug || os.Getenv("REEARTH_ACCOUNTS_DEV") == "true" {
		debugMonitor = &event.CommandMonitor{
			Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
				log.Errorf("MongoDB Command Failed: %s - Duration: %v - Error: %s",
					evt.CommandName, evt.Duration, evt.Failure)
			},
			Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
				// Only log slow queries or critical operations
				if evt.Duration > time.Millisecond*100 ||
					evt.CommandName == "createIndexes" ||
					evt.CommandName == "dropIndexes" ||
					evt.CommandName == "drop" {
					log.Debugf("MongoDB Command: %s - Duration: %v - Reply: %v",
						evt.CommandName, evt.Duration, evt.Reply)
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
		log.Fatalc(ctx, fmt.Sprintf("repo initialization error: %+v", err))
	}

	// Init repositories
	repos, gateways := initReposAndGateways(ctx, client, conf)

	// Check if migration mode
	// Once the permission check migration is complete, it will be deleted.
	if os.Getenv("RUN_MIGRATION") == "true" {
		clientx := mongox.NewClient(conf.DBName, client)
		db := clientx.Database()

		lock, lockErr := mongorepo.NewLock(db.Collection("locks"))
		if lockErr != nil {
			log.Fatalf("failed to create lock: %v", lockErr)
		}

		if migrationErr := runMigration(ctx, repos); migrationErr != nil {
			log.Fatal(migrationErr)
		}

		if migrationErr := migration.Do(ctx, clientx, mongorepo.NewConfig(db.Collection("config"), lock)); migrationErr != nil {
			log.Fatalf("failed to run migration: %v", migrationErr)
		}
		return
	}

	// Cerbos
	var opts []cerbos.Opt
	if !conf.CerbosUseSSL {
		opts = append(opts, cerbos.WithPlaintext(), cerbos.WithTLSInsecure())
	}

	cerbosClient, err := cerbos.New(conf.CerbosHost, opts...)
	if err != nil {
		log.Fatalf("Failed to create cerbos client: %v", err)
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
	defer log.Infoc(ctx, "Server shutdown")

	debugLog := ""
	if w.appServer.Debug {
		debugLog += " with debug mode"
	}
	log.Infof("server started%s at http://%s\n", debugLog, w.address)

	go func() {
		err := w.appServer.StartH2CServer(w.address, &http2.Server{})
		log.Fatalc(ctx, err.Error())
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}

func (w *WebServer) Serve(l net.Listener) error {
	return w.appServer.Server.Serve(l)
}

func (w *WebServer) Shutdown(ctx context.Context) error {
	return w.appServer.Shutdown(ctx)
}

// chainMongoMonitors composes multiple mongo CommandMonitors into one.
// nil monitors are ignored. Returns nil if all inputs are nil.
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
