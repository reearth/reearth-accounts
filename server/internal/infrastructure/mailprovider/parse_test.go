package mailprovider

import (
	"testing"

	"github.com/reearth/reearthx/mailer"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	sendgrid := Env{Provider: "sendgrid", FromEmail: "noreply@example.com", FromName: "Accounts", SendGridAPIKey: "sg-key"}

	tests := []struct {
		name    string
		env     Env
		dev     bool
		want    Settings
		wantErr string
	}{
		{name: "unset drops", want: Settings{Mode: ModeDrop}},
		{name: "unset in dev prints", dev: true, want: Settings{Mode: ModeDevPrint}},
		{
			name: "sendgrid complete",
			env:  sendgrid,
			want: Settings{Mode: ModeProvider, provider: mailer.Config{
				Mailer:   "sendgrid",
				SendGrid: mailer.SendGridConfig{API: "sg-key", Email: "noreply@example.com", Name: "Accounts"},
			}},
		},
		{
			name:    "sendgrid without api key",
			env:     Env{Provider: "sendgrid", FromEmail: "noreply@example.com"},
			wantErr: "REEARTH_ACCOUNTS_SENDGRID_API_KEY is required when REEARTH_ACCOUNTS_MAILER=sendgrid",
		},
		{
			name:    "sendgrid with whitespace-only api key",
			env:     Env{Provider: "sendgrid", FromEmail: "noreply@example.com", SendGridAPIKey: " \t "},
			wantErr: "REEARTH_ACCOUNTS_SENDGRID_API_KEY is required when REEARTH_ACCOUNTS_MAILER=sendgrid",
		},
		{
			name: "sendgrid values are trimmed",
			env:  Env{Provider: "sendgrid", FromEmail: " noreply@example.com\n", FromName: "Accounts", SendGridAPIKey: " sg-key "},
			want: Settings{Mode: ModeProvider, provider: mailer.Config{
				Mailer:   "sendgrid",
				SendGrid: mailer.SendGridConfig{API: "sg-key", Email: "noreply@example.com", Name: "Accounts"},
			}},
		},
		{
			name:    "sendgrid without from email",
			env:     Env{Provider: "sendgrid", SendGridAPIKey: "sg-key"},
			wantErr: "REEARTH_ACCOUNTS_MAILER_FROM_EMAIL is required when REEARTH_ACCOUNTS_MAILER=sendgrid",
		},
		{
			name: "ses",
			env:  Env{Provider: "ses", FromEmail: "noreply@example.com", FromName: "Accounts"},
			want: Settings{Mode: ModeProvider, provider: mailer.Config{
				Mailer: "ses",
				SES:    mailer.SESConfig{Email: "noreply@example.com", Name: "Accounts"},
			}},
		},
		{
			name:    "ses without from email",
			env:     Env{Provider: "ses"},
			wantErr: "REEARTH_ACCOUNTS_MAILER_FROM_EMAIL is required when REEARTH_ACCOUNTS_MAILER=ses",
		},
		{
			name:    "ses with whitespace-only from email",
			env:     Env{Provider: "ses", FromEmail: "   "},
			wantErr: "REEARTH_ACCOUNTS_MAILER_FROM_EMAIL is required when REEARTH_ACCOUNTS_MAILER=ses",
		},
		{name: "provider set in dev still sends", env: sendgrid, dev: true, want: Settings{Mode: ModeProvider, provider: mailer.Config{
			Mailer:   "sendgrid",
			SendGrid: mailer.SendGridConfig{API: "sg-key", Email: "noreply@example.com", Name: "Accounts"},
		}}},
	}
	for _, name := range []string{"smtp", "cloudflare", "direct", "logger", "sendgird", "SendGrid"} {
		tests = append(tests, struct {
			name    string
			env     Env
			dev     bool
			want    Settings
			wantErr string
		}{
			name:    name + " refused",
			env:     Env{Provider: name, FromEmail: "noreply@example.com", SendGridAPIKey: "sg-key"},
			wantErr: `REEARTH_ACCOUNTS_MAILER: unsupported provider "` + name + `"`,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.env, tt.dev)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
