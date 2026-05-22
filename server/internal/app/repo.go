package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/auth0"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/storage"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/mongo"
)

// initGateways builds the shared gateway container (storage, mailer, auth) used
// by every backend.
func initGateways(ctx context.Context, conf *Config) *gateway.Container {
	str, err := storage.NewGCPStorage(&storage.Config{
		IsLocal:          conf.StorageIsLocal,
		BucketName:       conf.StorageBucketName,
		EmulatorEnabled:  conf.StorageEmulatorEnabled,
		EmulatorEndpoint: conf.StorageEmulatorEndpoint,
	})
	if err != nil {
		log.Fatalf("Failed to init storage: %+v\n", err)
	}

	return &gateway.Container{
		Mailer:        mailer.New(ctx, &mailer.Config{}),
		Authenticator: auth0.New(conf.Auth0.Domain, conf.Auth0.ClientID, conf.Auth0.ClientSecret),
		Storage:       str,
	}
}

// initPostgresReposAndGateways wires the PostgreSQL-backed repo container.
func initPostgresReposAndGateways(ctx context.Context, pool *pgxpool.Pool, conf *Config) (*repo.Container, *gateway.Container) {
	repos, err := postgres.New(ctx, pool, []user.Repo{})
	if err != nil {
		log.Fatalf("Failed to init postgres: %+v\n", err)
	}
	return repos, initGateways(ctx, conf)
}

func initReposAndGateways(ctx context.Context, client *mongo.Client, conf *Config) (*repo.Container, *gateway.Container) {
	txAvailable := mongox.IsTransactionAvailable(conf.DB)

	repos, err := mongorepo.New(ctx, client.Database(conf.DBName), txAvailable, false, []user.Repo{})
	if err != nil {
		log.Fatalf("Failed to init mongo: %+v\n", err)
	}

	return repos, initGateways(ctx, conf)
}
