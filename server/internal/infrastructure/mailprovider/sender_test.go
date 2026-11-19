package mailprovider

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	address = "someone@example.com"
	link    = "https://accounts.example/?user-verification-token=SECRET"
	subject = "email verification"
)

func envelope(uid id.UserID) mail.Envelope {
	return mail.Envelope{
		Message: mail.Raw(mail.KindVerification, subject, "open "+link, `<a href="`+link+`">open</a>`),
		To:      mail.ToUser(uid, address, "Someone"),
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })
	return &buf
}

func TestSender_Provider_OneContactPerSend(t *testing.T) {
	m := mailer.NewMock()
	s := &sender{m: m, mode: ModeProvider}
	uid := id.NewUserID()

	require.NoError(t, s.Send(context.Background(), envelope(uid)))
	require.NoError(t, s.Send(context.Background(), envelope(uid)))

	mails := m.Mails()
	require.Len(t, mails, 2)
	for _, got := range mails {
		assert.Equal(t, []mailer.Contact{{Email: address, Name: "Someone"}}, got.To)
		assert.Equal(t, subject, got.Subject)
		assert.Equal(t, "open "+link, got.PlainContent)
	}
	assert.True(t, s.Deliverable())
}

type failingMailer struct{ err error }

func (f failingMailer) SendMail(context.Context, []mailer.Contact, string, string, string) error {
	return f.err
}

func TestSender_Provider_ErrorIsSendErrorWithoutAddress(t *testing.T) {
	cause := errors.New("invalid email " + address)
	s := &sender{m: failingMailer{err: cause}, mode: ModeProvider}

	err := s.Send(context.Background(), envelope(id.NewUserID()))

	var se *mail.SendError
	require.ErrorAs(t, err, &se)
	assert.False(t, se.Permanent)
	assert.ErrorIs(t, err, cause)
	assert.NotContains(t, err.Error(), address)
}

func TestSender_Drop_LogsKindAndUserOnly(t *testing.T) {
	buf := captureLog(t)
	var out bytes.Buffer
	s := &sender{mode: ModeDrop, out: &out}
	uid := id.NewUserID()

	require.NoError(t, s.Send(context.Background(), envelope(uid)))

	logged := buf.String()
	assert.Contains(t, logged, "mail dropped: no provider configured")
	assert.Contains(t, logged, `"mail_kind": "verification"`)
	assert.Contains(t, logged, `"user_id": "`+uid.String()+`"`)
	for _, secret := range []string{address, "Someone", link, subject} {
		assert.NotContains(t, logged, secret)
	}
	assert.Empty(t, out.String())
	assert.False(t, s.Deliverable())
}

func TestSender_DevPrint_PrintsTextBody(t *testing.T) {
	buf := captureLog(t)
	var out bytes.Buffer
	s := &sender{mode: ModeDevPrint, out: &out}

	require.NoError(t, s.Send(context.Background(), envelope(id.NewUserID())))

	assert.Equal(t, "open "+link+"\n", out.String())
	assert.Contains(t, buf.String(), "mail dropped: no provider configured")
	assert.NotContains(t, buf.String(), link)
	assert.False(t, s.Deliverable())
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	buf := captureLog(t)

	drop, err := New(ctx, Settings{})
	require.NoError(t, err)
	assert.False(t, drop.Deliverable())
	assert.NotContains(t, buf.String(), "mailer:", "no provider must not build a reearthx mailer")

	sg, err := Parse(Env{Provider: "sendgrid", FromEmail: "noreply@example.com", SendGridAPIKey: "sg-key"}, false)
	require.NoError(t, err)
	m, err := New(ctx, sg)
	require.NoError(t, err)
	assert.True(t, m.Deliverable())
}

func TestNew_NilMailerFailsStartup(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/missing")
	t.Setenv("AWS_PROFILE", "accounts-test-missing-profile")
	ses, err := Parse(Env{Provider: "ses", FromEmail: "noreply@example.com"}, false)
	require.NoError(t, err)

	m, err := New(context.Background(), ses)

	assert.Nil(t, m)
	assert.ErrorContains(t, err, "REEARTH_ACCOUNTS_MAILER: ses failed to initialize")
}
