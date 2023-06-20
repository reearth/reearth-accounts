package app

import (
	"context"
	"fmt"
	"time"

	"github.com/reearth/reearthx/account/accountinfrastructure/accountmongo"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func initReposAndGateways(ctx context.Context, conf *Config, debug bool) (*accountrepo.Container, *accountgateway.Container) {
	// Mongo
	client, err := mongo.Connect(
		ctx,
		options.Client().
			ApplyURI(conf.DB).
			SetConnectTimeout(time.Second*10))
	if err != nil {
		log.Fatalln(fmt.Sprintf("repo initialization error: %+v", err))
	}

	acRepos, err := accountmongo.New(ctx, client, "reearth", true)
	if err != nil {
		log.Fatalln(fmt.Sprintf("Failed to init mongo: %+v", err))
	}

	acGateways := &accountgateway.Container{
		Mailer: mailer.New(ctx, &mailer.Config{}),
	}

	return acRepos, acGateways
}
