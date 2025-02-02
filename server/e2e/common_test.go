package e2e

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"

	"github.com/eukarya-inc/reearth-dashboard/internal/app"
	infraCerbos "github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/cerbos"
	"github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/memory"
	mongorepo "github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/mongo"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/gateway"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/labstack/gommon/log"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/account/accountinfrastructure/accountmemory"
	"github.com/reearth/reearthx/account/accountinfrastructure/accountmongo"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/samber/lo"
)

var (
	uID = user.NewID()
)

type Seeder func(ctx context.Context, r *accountrepo.Container) error

func init() {
	mongotest.Env = "REEARTH_DB"
}

func StartServer(t *testing.T, cfg *app.Config, useMongo bool, seeder Seeder) (*httpexpect.Expect, *accountrepo.Container) {
	e, r := StartServerAndRepos(t, cfg, useMongo, seeder)
	return e, r
}

func StartServerAndRepos(t *testing.T, cfg *app.Config, useMongo bool, seeder Seeder) (*httpexpect.Expect, *accountrepo.Container) {
	ctx := context.Background()

	var accountRepos *accountrepo.Container
	var repos *repo.Container

	if useMongo {
		db := mongorepo.Connect(t)(t)
		accountRepos = lo.Must(accountmongo.New(ctx, db.Client(), db.Name(), false, false, nil))

		var err error
		repos, err = mongorepo.New(ctx, db, accountRepos, false)
		if err != nil {
			log.Fatalf("Failed to init mongo: %+v\n", err)
		}
	} else {
		accountRepos = accountmemory.New()
		repos = memory.New()
	}

	if seeder != nil {
		if err := seeder(ctx, accountRepos); err != nil {
			t.Fatalf("failed to seed the db: %s", err)
		}
	}

	return StartServerWithRepos(
		t,
		cfg,
		repos,
		accountRepos,
	), accountRepos
}

func StartServerWithRepos(
	t *testing.T,
	cfg *app.Config,
	repos *repo.Container,
	accountrepos *accountrepo.Container,
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
		Config:       cfg,
		Repos:        repos,
		AccountRepos: accountrepos,
		Gateways: &accountgateway.Container{
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
