package app

import (
	"context"
	"net"
	"os"
	"os/signal"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/labstack/echo/v4"
	infraCerbos "github.com/reearth/reearth-accounts/internal/infrastructure/cerbos"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/log"
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

	// Init repositories
	repos, accountRepos, gateways := initReposAndGateways(ctx, conf)

	// Check if migration mode
	// Once the permission check migration is complete, it will be deleted.
	if os.Getenv("RUN_MIGRATION") == "true" {
		if err := runMigration(ctx, repos, accountRepos); err != nil {
			log.Fatal(err)
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
		AccountRepos:  accountRepos,
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
	AccountRepos  *accountrepo.Container
	Gateways      *accountgateway.Container
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
