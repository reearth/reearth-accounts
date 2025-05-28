package app

import (
	"context"
	"fmt"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/auth0"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/pkg/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/usecase/repo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const databaseName = "reearth-account"

func initReposAndGateways(ctx context.Context, conf *Config) (*repo.Container, *gateway.Container) {
	// Mongo
	client, err := mongo.Connect(
		ctx,
		options.Client().
			ApplyURI(conf.DB).
			SetConnectTimeout(time.Second*10))
	if err != nil {
		log.Fatalc(ctx, fmt.Sprintf("repo initialization error: %+v", err))
	}

	txAvailable := mongox.IsTransactionAvailable(conf.DB)

	repos, err := mongorepo.New(ctx, client.Database(databaseName), txAvailable, false, []repo.User{})
	if err != nil {
		log.Fatalf("Failed to init mongo: %+v\n", err)
	}

	acGateways := &gateway.Container{
		Mailer:        mailer.New(ctx, &mailer.Config{}),
		Authenticator: auth0.New(conf.Auth0.Domain, conf.Auth0.ClientID, conf.Auth0.ClientSecret),
	}

	return repos, acGateways
}
