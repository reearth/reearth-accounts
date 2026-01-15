package migration

import (
	"context"
	"errors"
	"sync"

	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/usecasex/migration"
)

// To add a new migration, do the following command:
// go run github.com/reearth/reearthx/tools migrategen -d internal/infrastructure/mongo/migration -t DBClient -n "FooBar"

type DBClient = *mongox.Client

func Do(ctx context.Context, db *mongox.Client, cfg config.Repo) error {
	return migration.NewClient(
		db,
		NewConfig(cfg),
		migrations,
		0,
	).Migrate(ctx)
}

type Config struct {
	c       config.Repo
	locked  bool
	current config.Config
	m       sync.Mutex
}

func NewConfig(c config.Repo) *Config {
	return &Config{c: c}
}

func (c *Config) Begin(ctx context.Context) error {
	c.m.Lock()
	defer c.m.Unlock()

	conf, err := c.c.LockAndLoad(ctx)
	if conf != nil {
		c.current = *conf
	}
	if err == nil {
		c.locked = true
	}
	return err
}

func (c *Config) End(ctx context.Context) error {
	c.m.Lock()
	defer c.m.Unlock()

	if err := c.c.Unlock(ctx); err != nil {
		return err
	}
	c.locked = false
	return nil
}

func (c *Config) Current(ctx context.Context) (migration.Key, error) {
	if !c.locked {
		return 0, errors.New("config is not locked")
	}
	return c.current.Migration, nil
}

func (c *Config) Save(ctx context.Context, key migration.Key) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.current.Migration = key
	return c.c.Save(ctx, &c.current)
}
