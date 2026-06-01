package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/auth0"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/cip"
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

	mailerInstance := mailer.New(ctx, &mailer.Config{})

	// Build per-provider authenticators so management calls (UpdateUser,
	// ResendVerificationEmail) are routed by each user's auth record provider
	// rather than swapping a single authenticator globally. This keeps Auth0
	// subs going to Auth0 and CIP subs going to Firebase when both coexist.
	authenticators := map[gateway.Provider]gateway.Authenticator{}
	if conf.Auth0.Domain != "" {
		authenticators[gateway.ProviderAuth0] = auth0.New(conf.Auth0.Domain, conf.Auth0.ClientID, conf.Auth0.ClientSecret)
	}
	if conf.GetAuthProvider() == "cip" {
		if conf.CIP.ProjectID == "" {
			log.Fatalf("REEARTH_ACCOUNTS_AUTH_PROVIDER=cip requires REEARTH_ACCOUNTS_CIP_PROJECT_ID")
		}
		cipAuth, cipErr := cip.New(ctx, cip.Params{
			ProjectID: conf.CIP.ProjectID,
			TenantID:  conf.CIP.TenantID,
		}, mailerInstance)
		if cipErr != nil {
			log.Fatalf("Failed to init CIP authenticator: %+v\n", cipErr)
		}
		authenticators[gateway.ProviderCIP] = cipAuth
	}

	return &gateway.Container{
		Mailer:         mailerInstance,
		Authenticators: authenticators,
		Storage:        str,
	}
}

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
