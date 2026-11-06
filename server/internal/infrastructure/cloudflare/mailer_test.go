package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okBody(delivered ...string) map[string]any {
	return map[string]any{
		"success": true,
		"result": map[string]any{
			"delivered":        delivered,
			"queued":           []string{},
			"permanent_bounces": []string{},
		},
	}
}

func TestNew_nilConf(t *testing.T) {
	_, err := New(nil)
	assert.ErrorContains(t, err, "conf must not be nil")
}

func TestNew_emptyToken(t *testing.T) {
	_, err := New(&Config{AccountID: "acc", SenderEmail: "s@example.com"})
	assert.ErrorContains(t, err, "APIToken must not be empty")
}

func TestNew_emptyAccountID(t *testing.T) {
	_, err := New(&Config{APIToken: "tok", SenderEmail: "s@example.com"})
	assert.ErrorContains(t, err, "AccountID must not be empty")
}

func TestNew_emptySenderEmail(t *testing.T) {
	_, err := New(&Config{APIToken: "tok", AccountID: "acc"})
	assert.ErrorContains(t, err, "SenderEmail must not be empty")
}

func TestNew_sendURLFormattedWithAccountID(t *testing.T) {
	m, err := New(&Config{APIToken: "tok", AccountID: "acc123", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.cloudflare.com/client/v4/accounts/acc123/email/sending/send", m.(*mailer).sendURL)
}

func TestNew_defaultTimeout(t *testing.T) {
	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	assert.Equal(t, defaultHTTPTimeout, m.(*mailer).client.Timeout)
}

func TestNew_customTimeout(t *testing.T) {
	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com", HTTPTimeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, m.(*mailer).client.Timeout)
}

func TestMailer_SendMail(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(okBody("to@example.com"))
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "test-token", AccountID: "acc123", SenderEmail: "sender@example.com", SenderName: "Sender"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	require.NoError(t, m.SendMail(context.Background(),
		[]Contact{{Email: "to@example.com", Name: "To User"}},
		"Hello", "plain body", "<b>html body</b>",
	))

	assert.Equal(t, map[string]any{
		"from":    map[string]any{"address": "sender@example.com", "name": "Sender"},
		"to":      []any{map[string]any{"address": "to@example.com", "name": "To User"}},
		"subject": "Hello",
		"text":    "plain body",
		"html":    "<b>html body</b>",
	}, captured)
}

func TestMailer_SendMail_perRecipient(t *testing.T) {
	var bodies []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(okBody("x"))
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	require.NoError(t, m.SendMail(context.Background(),
		[]Contact{{Email: "a@example.com"}, {Email: "b@example.com"}},
		"sub", "plain", "html",
	))

	assert.Len(t, bodies, 2)
	for _, b := range bodies {
		assert.Len(t, b["to"].([]any), 1, "each request must have exactly one recipient")
	}
}

func TestMailer_SendMail_emptyTo(t *testing.T) {
	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	assert.ErrorContains(t, m.SendMail(context.Background(), nil, "sub", "plain", "html"), "to must not be empty")
}

func TestMailer_SendMail_permanentBounce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  map[string]any{"delivered": []string{}, "queued": []string{}, "permanent_bounces": []string{"to@example.com"}},
		})
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	assert.ErrorContains(t, m.SendMail(context.Background(), []Contact{{Email: "to@example.com"}}, "sub", "plain", "html"), "permanent bounce")
}

func TestMailer_SendMail_errorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"errors":  []map[string]any{{"code": 10102, "message": "Token does not have send permission"}},
		})
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	err = m.SendMail(context.Background(), []Contact{{Email: "x@example.com"}}, "sub", "plain", "html")
	assert.ErrorContains(t, err, "10102")
	assert.ErrorContains(t, err, "Token does not have send permission")
}

func TestMailer_SendMail_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	assert.ErrorContains(t, m.SendMail(context.Background(), []Contact{{Email: "x@example.com"}}, "sub", "plain", "html"), "status 500")
}
