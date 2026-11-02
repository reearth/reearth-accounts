package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_emptyToken(t *testing.T) {
	_, err := New(&Config{})
	assert.ErrorContains(t, err, "API Token must not be empty")
}

func TestNew_validToken(t *testing.T) {
	_, err := New(&Config{APIToken: "tok", AccountID: "acc"})
	assert.NoError(t, err)
}

func TestMailer_SendMail(t *testing.T) {
	var captured cfSendPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m, err := New(&Config{
		APIToken:    "test-token",
		AccountID:   "acc123",
		SenderEmail: "sender@example.com",
		SenderName:  "Sender",
	})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	err = m.SendMail(context.Background(),
		[]Contact{{Email: "to@example.com", Name: "To User"}},
		"Hello",
		"plain body",
		"<b>html body</b>",
	)
	require.NoError(t, err)

	assert.Equal(t, "sender@example.com", captured.From.Address)
	assert.Equal(t, "Sender", captured.From.Name)
	assert.Equal(t, "Hello", captured.Subject)
	assert.Equal(t, "plain body", captured.Text)
	assert.Equal(t, "<b>html body</b>", captured.HTML)
	require.Len(t, captured.To, 1)
	assert.Equal(t, "to@example.com", captured.To[0].Address)
	assert.Equal(t, "To User", captured.To[0].Name)
}

func TestMailer_SendMail_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m, err := New(&Config{APIToken: "tok", AccountID: "acc", SenderEmail: "s@example.com"})
	require.NoError(t, err)
	m.(*mailer).sendURL = srv.URL

	err = m.SendMail(context.Background(), []Contact{{Email: "x@example.com"}}, "sub", "plain", "html")
	assert.ErrorContains(t, err, "unexpected status 500")
}
