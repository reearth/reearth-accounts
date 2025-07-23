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
	infraCerbos "github.com/reearth/reearth-accounts/internal/infrastructure/cerbos"
	mongorepo "github.com/reearth/reearth-accounts/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/migration"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	// Init MongoDB client
	client, err := mongo.Connect(
		ctx,
		options.Client().
			ApplyURI(conf.DB).
			SetConnectTimeout(time.Second*10))
	if err != nil {
		log.Fatalc(ctx, fmt.Sprintf("repo initialization error: %+v", err))
	}

	// Init repositories
	repos, gateways := initReposAndGateways(ctx, client, conf)

	// Check if migration mode
	// Once the permission check migration is complete, it will be deleted.
	if os.Getenv("RUN_MIGRATION") == "true" {
		clientx := mongox.NewClient("reearth-accounts", client)
		db := clientx.Database()

		lock, lockErr := mongorepo.NewLock(db.Collection("locks"))
		if lockErr != nil {
			log.Fatalf("failed to create lock: %v", lockErr)
		}

		if migrationErr := runMigration(ctx, repos); migrationErr != nil {
			log.Fatal(migrationErr)
		}

		if migrationErr := migration.Do(ctx, clientx, mongorepo.NewConfig(db.Collection("config"), lock)); err != nil {
			log.Fatalf("failed to run migration: %v", migrationErr)
		}
		return
	}

	// Cerbos
	var opts []cerbos.Opt
	if os.Getenv("REEARTH_ACCOUNTS_DEV") == "true" {
		opts = append(opts, cerbos.WithPlaintext())
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
	if port == "" {
		port = "8080"
	}

	address := "0.0.0.0:" + port
	if cfg.Debug {
		address = "localhost:" + port
	}

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
