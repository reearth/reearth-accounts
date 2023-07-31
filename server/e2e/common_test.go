package e2e

import (
	"context"
	"net"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-account/internal/app"
	"github.com/reearth/reearthx/account/accountinfrastructure/accountmemory"
	"github.com/reearth/reearthx/account/accountinfrastructure/accountmongo"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/samber/lo"
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

	if useMongo {
		db := mongotest.Connect(t)(t)
		accountRepos = lo.Must(accountmongo.New(ctx, db.Client(), db.Name(), false, false))
	} else {
		accountRepos = accountmemory.New()
	}

	if seeder != nil {
		if err := seeder(ctx, accountRepos); err != nil {
			t.Fatalf("failed to seed the db: %s", err)
		}
	}

	return StartServerWithRepos(t, cfg, accountRepos), accountRepos
}

func StartServerWithRepos(t *testing.T, cfg *app.Config, accountrepos *accountrepo.Container) *httpexpect.Expect {
	t.Helper()

	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("server failed to listen: %v", err)
	}

	srv := app.NewServer(ctx, &app.ServerConfig{
		Config: cfg,
		Repos:  accountrepos,
		Gateways: &accountgateway.Container{
			Mailer: mailer.New(ctx, &mailer.Config{}),
		},
		Debug: true,
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
