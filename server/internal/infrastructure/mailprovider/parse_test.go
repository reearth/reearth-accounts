package mailprovider

import (
	"testing"

	"github.com/reearth/reearthx/mailer"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	sendgrid := Env{Provider: "sendgrid", FromEmail: "noreply@example.com", FromName: "Accounts", SendGridAPIKey: "sg-key"}

	cloudflare := Env{Provider: "cloudflare", FromEmail: "noreply@example.com", FromName: "Accounts", CloudflareAccountID: "acc-id", CloudflareAPIToken: "cf-token"}

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
		{
			name: "cloudflare complete",
			env:  cloudflare,
			want: Settings{Mode: ModeProvider, provider: mailer.Config{
				Mailer:     "cloudflare",
				Cloudflare: mailer.CloudflareConfig{Email: "noreply@example.com", Name: "Accounts", AccountID: "acc-id", APIToken: "cf-token"},
			}},
		},
		{
			name: "cloudflare values are trimmed",
			env:  Env{Provider: "cloudflare", FromEmail: " noreply@example.com ", CloudflareAccountID: " acc-id\n", CloudflareAPIToken: " cf-token "},
			want: Settings{Mode: ModeProvider, provider: mailer.Config{
				Mailer:     "cloudflare",
				Cloudflare: mailer.CloudflareConfig{Email: "noreply@example.com", AccountID: "acc-id", APIToken: "cf-token"},
			}},
		},
		{
			name:    "cloudflare without account id",
			env:     Env{Provider: "cloudflare", FromEmail: "noreply@example.com", CloudflareAPIToken: "cf-token"},
			wantErr: "REEARTH_ACCOUNTS_CLOUDFLARE_ACCOUNT_ID is required when REEARTH_ACCOUNTS_MAILER=cloudflare",
		},
		{
			name:    "cloudflare without token",
			env:     Env{Provider: "cloudflare", FromEmail: "noreply@example.com", CloudflareAccountID: "acc-id"},
			wantErr: "REEARTH_ACCOUNTS_CLOUDFLARE_API_TOKEN is required when REEARTH_ACCOUNTS_MAILER=cloudflare",
		},
		{
			name:    "cloudflare with blank token",
			env:     Env{Provider: "cloudflare", FromEmail: "noreply@example.com", CloudflareAccountID: "acc-id", CloudflareAPIToken: " \t "},
			wantErr: "REEARTH_ACCOUNTS_CLOUDFLARE_API_TOKEN is required when REEARTH_ACCOUNTS_MAILER=cloudflare",
		},
		{
			name:    "cloudflare without from email",
			env:     Env{Provider: "cloudflare", CloudflareAccountID: "acc-id", CloudflareAPIToken: "cf-token"},
			wantErr: "REEARTH_ACCOUNTS_MAILER_FROM_EMAIL is required when REEARTH_ACCOUNTS_MAILER=cloudflare",
		},
		{name: "provider set in dev still sends", env: sendgrid, dev: true, want: Settings{Mode: ModeProvider, provider: mailer.Config{
			Mailer:   "sendgrid",
			SendGrid: mailer.SendGridConfig{API: "sg-key", Email: "noreply@example.com", Name: "Accounts"},
		}}},
	}
	for _, name := range []string{"smtp", "direct", "logger", "sendgird", "SendGrid"} {
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
