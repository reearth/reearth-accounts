package pgdoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/policy"
)

type ConfigRow struct {
	Migration     int64
	AuthCert      string
	AuthKey       string
	DefaultPolicy *string
}

func NewConfigRow(c config.Config) ConfigRow {
	row := ConfigRow{Migration: c.Migration}
	if c.Auth != nil {
		row.AuthCert = c.Auth.Cert
		row.AuthKey = c.Auth.Key
	}
	if c.DefaultPolicy != nil {
		s := c.DefaultPolicy.String()
		row.DefaultPolicy = &s
	}
	return row
}

func (r ConfigRow) Model() *config.Config {
	cfg := &config.Config{Migration: r.Migration}
	if r.AuthCert != "" || r.AuthKey != "" {
		cfg.Auth = &config.Auth{Cert: r.AuthCert, Key: r.AuthKey}
	}
	if r.DefaultPolicy != nil && *r.DefaultPolicy != "" {
		p := policy.ID(*r.DefaultPolicy)
		cfg.DefaultPolicy = &p
	}
	return cfg
}
