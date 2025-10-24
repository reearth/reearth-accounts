package e2e

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/internal/app"
	infraCerbos "github.com/reearth/reearth-accounts/server/internal/infrastructure/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox/mongotest"
)

var (
	uID = user.NewID()
)

type Seeder func(ctx context.Context, r *repo.Container) error

func init() {
	mongotest.Env = "REEARTH_DB"
}

func StartServer(t *testing.T, cfg *app.Config, useMongo bool, seeder Seeder) (*httpexpect.Expect, *repo.Container) {
	e, r := StartServerAndRepos(t, cfg, useMongo, seeder)
	return e, r
}

func StartServerAndRepos(t *testing.T, cfg *app.Config, useMongo bool, seeder Seeder) (*httpexpect.Expect, *repo.Container) {
	ctx := context.Background()
	var repos *repo.Container

	if useMongo {
		db := mongorepo.Connect(t)(t)

		var err error
		repos, err = mongorepo.New(ctx, db, false, false, nil)
		if err != nil {
			log.Fatalf("Failed to init mongo: %+v\n", err)
		}
	} else {
		repos = memory.New()
	}

	if seeder != nil {
		if err := seeder(ctx, repos); err != nil {
			t.Fatalf("failed to seed the db: %s", err)
		}
	}

	return StartServerWithRepos(
		t,
		cfg,
		repos,
	), repos
}

func StartServerWithRepos(
	t *testing.T,
	cfg *app.Config,
	repos *repo.Container,
) *httpexpect.Expect {
	t.Helper()

	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("server failed to listen: %v", err)
	}

	// Cerbos
	var cerbosAdapter gateway.CerbosGateway
	if cfg.CerbosHost != "" {
		cerbosClient, err := cerbos.New(cfg.CerbosHost, cerbos.WithPlaintext())
		if err != nil {
			log.Fatalf("Failed to create cerbos client: %v", err)
		}
		cerbosAdapter = infraCerbos.NewCerbosAdapter(cerbosClient)
	}

	srv := app.NewServer(ctx, &app.ServerConfig{
		Config: cfg,
		Repos:  repos,
		Gateways: &gateway.Container{
			Mailer: mailer.New(ctx, &mailer.Config{}),
		},
		Debug:         true,
		CerbosAdapter: cerbosAdapter,
	})

	ch := make(chan error)
	go func() {
		if err := srv.Serve(l); err != http.ErrServerClosed {
			ch <- err
		}
		close(ch)
	}()
	t.Cleanup(func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Fatalf("server shutdown: %v", err)
		}

		if err := <-ch; err != nil {
			t.Fatalf("server serve: %v", err)
		}
	})
	return httpexpect.Default(t, "http://"+l.Addr().String())
}
