package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	cf "github.com/cloudflare/cloudflare-go"
)

// defaultSendURL is the Cloudflare Email Service endpoint.
// https://developers.cloudflare.com/email-service/api/send-emails/rest-api/
const defaultSendURL = "https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send"

// Mailer sends transactional email via the Cloudflare Email Service REST API.
type Mailer interface {
	SendMail(ctx context.Context, to []Contact, subject, plainContent, htmlContent string) error
}

// Contact is a named email address.
type Contact struct {
	Email string
	Name  string
}

// Config holds the credentials and sender identity for the Cloudflare mailer.
type Config struct {
	AccountID   string
	APIToken    string
	SenderEmail string
	SenderName  string
}

type mailer struct {
	conf    *Config
	api     *cf.API
	client  *http.Client
	sendURL string // formatted with AccountID; overridable in tests
}

// New creates a Mailer backed by the Cloudflare Email Service.
// Returns an error if the Cloudflare API client cannot be initialized.
func New(conf *Config) (Mailer, error) {
	api, err := cf.NewWithAPIToken(conf.APIToken)
	if err != nil {
		return nil, fmt.Errorf("cloudflare mailer: init api client: %w", err)
	}

	return &mailer{
		conf:    conf,
		api:     api,
		client:  &http.Client{},
		sendURL: fmt.Sprintf(defaultSendURL, conf.AccountID),
	}, nil
}

// cfAddress mirrors the Cloudflare Email Service address object.
type cfAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type cfSendPayload struct {
	From    cfAddress   `json:"from"`
	To      []cfAddress `json:"to"`
	Subject string      `json:"subject"`
	HTML    string      `json:"html,omitempty"`
	Text    string      `json:"text,omitempty"`
}

func (m *mailer) SendMail(ctx context.Context, to []Contact, subject, plainContent, htmlContent string) error {
	recipients := make([]cfAddress, 0, len(to))
	for _, c := range to {
		recipients = append(recipients, cfAddress{Address: c.Email, Name: c.Name})
	}

	payload := cfSendPayload{
		From:    cfAddress{Address: m.conf.SenderEmail, Name: m.conf.SenderName},
		To:      recipients,
		Subject: subject,
		Text:    plainContent,
		HTML:    htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cloudflare mailer: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.sendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudflare mailer: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.conf.APIToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare mailer: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare mailer: unexpected status %d", resp.StatusCode)
	}

	return nil
}
