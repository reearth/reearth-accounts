package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultSendURL     = "https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send"
	defaultHTTPTimeout = 10 * time.Second
)

// Mailer sends transactional email via the Cloudflare Email Service REST API.
// Independent of reearthx/mailer.Mailer; wiring into gateway.Container requires a thin adapter.
type Mailer interface {
	SendMail(ctx context.Context, to []Contact, subject, plainContent, htmlContent string) error
}

type Contact struct {
	Email string
	Name  string
}

type Config struct {
	AccountID   string
	APIToken    string
	HTTPTimeout time.Duration // defaults to 10 s when zero
	SenderEmail string
	SenderName  string
}

type mailer struct {
	conf    Config
	client  *http.Client
	sendURL string
}

func New(conf *Config) (Mailer, error) {
	if conf == nil {
		return nil, fmt.Errorf("cloudflare mailer: conf must not be nil")
	}
	if conf.APIToken == "" {
		return nil, fmt.Errorf("cloudflare mailer: APIToken must not be empty")
	}
	if conf.AccountID == "" {
		return nil, fmt.Errorf("cloudflare mailer: AccountID must not be empty")
	}
	if conf.SenderEmail == "" {
		return nil, fmt.Errorf("cloudflare mailer: SenderEmail must not be empty")
	}

	timeout := conf.HTTPTimeout
	if timeout == 0 {
		timeout = defaultHTTPTimeout
	}

	return &mailer{
		conf:    *conf,
		client:  &http.Client{Timeout: timeout},
		sendURL: fmt.Sprintf(defaultSendURL, url.PathEscape(conf.AccountID)),
	}, nil
}

type cfAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type cfSendPayload struct {
	From    cfAddress   `json:"from"`
	HTML    string      `json:"html,omitempty"`
	Subject string      `json:"subject"`
	Text    string      `json:"text,omitempty"`
	To      []cfAddress `json:"to"`
}

type cfResponse struct {
	Errors  []cfError `json:"errors"`
	Result  *cfResult `json:"result"`
	Success bool      `json:"success"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfResult struct {
	Delivered       []string `json:"delivered"`
	PermanentBounces []string `json:"permanent_bounces"`
	Queued          []string `json:"queued"`
}

// SendMail sends one message per recipient to avoid leaking addresses across recipients.
func (m *mailer) SendMail(ctx context.Context, to []Contact, subject, plainContent, htmlContent string) error {
	if len(to) == 0 {
		return fmt.Errorf("cloudflare mailer: to must not be empty")
	}

	sender := cfAddress{Address: m.conf.SenderEmail, Name: m.conf.SenderName}
	for _, c := range to {
		if err := m.send(ctx, sender, c, subject, plainContent, htmlContent); err != nil {
			return err
		}
	}
	return nil
}

func (m *mailer) send(ctx context.Context, from cfAddress, to Contact, subject, plainContent, htmlContent string) error {
	payload := cfSendPayload{
		From:    from,
		HTML:    htmlContent,
		Subject: subject,
		Text:    plainContent,
		To:      []cfAddress{{Address: to.Email, Name: to.Name}},
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

	var envelope cfResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)

	if resp.StatusCode >= 300 || !envelope.Success {
		if decodeErr != nil || len(envelope.Errors) == 0 {
			return fmt.Errorf("cloudflare mailer: status %d: unknown error", resp.StatusCode)
		}
		return fmt.Errorf("cloudflare mailer: status %d: %s", resp.StatusCode, formatErrors(envelope.Errors))
	}

	if decodeErr != nil {
		return fmt.Errorf("cloudflare mailer: decode response: %w", decodeErr)
	}

	if r := envelope.Result; r != nil {
		if len(r.PermanentBounces) > 0 {
			return fmt.Errorf("cloudflare mailer: permanent bounce for %s", to.Email)
		}
		if len(r.Delivered) == 0 && len(r.Queued) == 0 {
			return fmt.Errorf("cloudflare mailer: %s was neither delivered nor queued", to.Email)
		}
	}

	return nil
}

func formatErrors(errs []cfError) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("%d: %s", e.Code, e.Message))
	}
	return strings.Join(parts, "; ")
}
