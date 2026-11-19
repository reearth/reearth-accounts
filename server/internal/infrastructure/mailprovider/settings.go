// Package mailprovider builds gateway.Mail from the environment on top of the
// reearthx mailer (ADR-0004 decisions 10 to 12).
package mailprovider

import (
	"fmt"

	"github.com/reearth/reearthx/mailer"
)

const (
	providerSendGrid = "sendgrid"
	providerSES      = "ses"
)

type Env struct {
	FromEmail      string `envconfig:"REEARTH_ACCOUNTS_MAILER_FROM_EMAIL"`
	FromName       string `envconfig:"REEARTH_ACCOUNTS_MAILER_FROM_NAME"`
	Provider       string `envconfig:"REEARTH_ACCOUNTS_MAILER"`
	SendGridAPIKey string `envconfig:"REEARTH_ACCOUNTS_SENDGRID_API_KEY"`
}

type Mode int

const (
	ModeDrop Mode = iota
	ModeDevPrint
	ModeProvider
)

// Settings is a validated Env. Its zero value drops every message.
type Settings struct {
	Mode     Mode
	provider mailer.Config
}

// Parse accepts only the providers accounts offers. smtp, cloudflare, direct
// and logger are refused like any unknown name, so a typo cannot fall back to
// the reearthx logger mailer.
func Parse(e Env, dev bool) (Settings, error) {
	switch e.Provider {
	case "":
		if dev {
			return Settings{Mode: ModeDevPrint}, nil
		}
		return Settings{Mode: ModeDrop}, nil
	case providerSendGrid:
		if err := requireFields(e.Provider,
			field{"REEARTH_ACCOUNTS_MAILER_FROM_EMAIL", e.FromEmail},
			field{"REEARTH_ACCOUNTS_SENDGRID_API_KEY", e.SendGridAPIKey},
		); err != nil {
			return Settings{}, err
		}
		return Settings{Mode: ModeProvider, provider: mailer.Config{
			Mailer:   providerSendGrid,
			SendGrid: mailer.SendGridConfig{API: e.SendGridAPIKey, Email: e.FromEmail, Name: e.FromName},
		}}, nil
	case providerSES:
		if err := requireFields(e.Provider, field{"REEARTH_ACCOUNTS_MAILER_FROM_EMAIL", e.FromEmail}); err != nil {
			return Settings{}, err
		}
		return Settings{Mode: ModeProvider, provider: mailer.Config{
			Mailer: providerSES,
			SES:    mailer.SESConfig{Email: e.FromEmail, Name: e.FromName},
		}}, nil
	default:
		return Settings{}, fmt.Errorf("REEARTH_ACCOUNTS_MAILER: unsupported provider %q (supported: %s, %s)", e.Provider, providerSendGrid, providerSES)
	}
}

type field struct {
	env   string
	value string
}

func requireFields(provider string, fields ...field) error {
	for _, f := range fields {
		if f.value == "" {
			return fmt.Errorf("%s is required when REEARTH_ACCOUNTS_MAILER=%s", f.env, provider)
		}
	}
	return nil
}
