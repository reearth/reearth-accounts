package app

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/infrastructure/auth0"
	mongorepo "github.com/reearth/reearth-accounts/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/mongo"
)

func initReposAndGateways(ctx context.Context, client *mongo.Client, conf *Config) (*repo.Container, *gateway.Container) {
	txAvailable := mongox.IsTransactionAvailable(conf.DB)

	repos, err := mongorepo.New(ctx, client.Database(conf.DBName), txAvailable, false, []repo.User{})
	if err != nil {
		log.Fatalf("Failed to init mongo: %+v\n", err)
	}

	acGateways := &gateway.Container{
		Mailer:        mailer.New(ctx, &mailer.Config{}),
		Authenticator: auth0.New(conf.Auth0.Domain, conf.Auth0.ClientID, conf.Auth0.ClientSecret),
	}

	return repos, acGateways
}
