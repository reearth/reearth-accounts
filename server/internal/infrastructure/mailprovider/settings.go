package mailprovider

import (
	"fmt"
	"strings"

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

type Settings struct {
	Mode     Mode
	provider mailer.Config
}

func Parse(e Env, dev bool) (Settings, error) {
	e.FromEmail = strings.TrimSpace(e.FromEmail)
	e.SendGridAPIKey = strings.TrimSpace(e.SendGridAPIKey)
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
