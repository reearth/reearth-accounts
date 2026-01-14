package config

import (
	"context"
)

type Repo interface {
	LockAndLoad(context.Context) (*Config, error)
	Save(context.Context, *Config) error
	SaveAuth(context.Context, *Auth) error
	SaveAndUnlock(context.Context, *Config) error
	Unlock(context.Context) error
}
