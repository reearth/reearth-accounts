package mongodoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/policy"
)

type ConfigDocument struct {
	Migration     int64      `json:"migration" jsonschema:"description=Current migration version number. Default: 0"`
	Auth          *Auth      `json:"auth" jsonschema:"description=Authentication certificates configuration. Default: null"`
	DefaultPolicy *policy.ID `json:"defaultpolicy" jsonschema:"description=Default policy ID. Default: null"`
}

type Auth struct {
	Cert string `json:"cert" jsonschema:"description=Auth certificate (PEM format). Default: \"\""`
	Key  string `json:"key" jsonschema:"description=Auth private key (PEM format). Default: \"\""`
}

func NewConfig(c config.Config) ConfigDocument {
	return ConfigDocument{
		Migration:     c.Migration,
		Auth:          NewConfigAuth(c.Auth),
		DefaultPolicy: c.DefaultPolicy,
	}
}

func NewConfigAuth(c *config.Auth) *Auth {
	if c == nil {
		return nil
	}
	return &Auth{
		Cert: c.Cert,
		Key:  c.Key,
	}
}

func (c *ConfigDocument) Model() *config.Config {
	if c == nil {
		return &config.Config{}
	}

	cfg := &config.Config{
		Migration:     c.Migration,
		DefaultPolicy: c.DefaultPolicy,
	}

	if c.Auth != nil {
		cfg.Auth = &config.Auth{
			Cert: c.Auth.Cert,
			Key:  c.Auth.Key,
		}
	}

	return cfg
}
