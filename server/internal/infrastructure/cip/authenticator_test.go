package cip

import (
	"context"
	"errors"
	"testing"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/mailer"
	"github.com/stretchr/testify/assert"
)

// fakeMailer implements mailer.Mailer for tests and records the last sent message.
type fakeMailer struct {
	sent    bool
	to      []mailer.Contact
	subject string
	text    string
	html    string
}

func (f *fakeMailer) SendMail(_ context.Context, to []mailer.Contact, subject, text, html string) error {
	f.sent = true
	f.to = to
	f.subject = subject
	f.text = text
	f.html = html
	return nil
}

// fakeAuthClient implements firebaseAuthClient for tests (no SDK / network).
type fakeAuthClient struct {
	updateCalls     []updateCall
	getUser         func(ctx context.Context, uid string) (*fbauth.UserRecord, error)
	verifyLinkEmail string
	verifyLinkErr   error
}

type updateCall struct {
	uid  string
	user *fbauth.UserToUpdate
}

func (f *fakeAuthClient) UpdateUser(ctx context.Context, uid string, u *fbauth.UserToUpdate) (*fbauth.UserRecord, error) {
	f.updateCalls = append(f.updateCalls, updateCall{uid: uid, user: u})
	if f.getUser != nil {
		return f.getUser(ctx, uid)
	}
	return &fbauth.UserRecord{UserInfo: &fbauth.UserInfo{UID: uid}}, nil
}

func (f *fakeAuthClient) GetUser(ctx context.Context, uid string) (*fbauth.UserRecord, error) {
	if f.getUser != nil {
		return f.getUser(ctx, uid)
	}
	return &fbauth.UserRecord{UserInfo: &fbauth.UserInfo{UID: uid}}, nil
}

func (f *fakeAuthClient) EmailVerificationLink(ctx context.Context, email string) (string, error) {
	f.verifyLinkEmail = email
	if f.verifyLinkErr != nil {
		return "", f.verifyLinkErr
	}
	return "https://verify.example/" + email, nil
}

func newTestAuthenticator(c firebaseAuthClient) *Authenticator {
	return &Authenticator{client: c, mailer: &fakeMailer{}}
}

func strptr(s string) *string { return &s }

func TestCIP_New_RequiresProjectID(t *testing.T) {
	_, err := New(context.Background(), Params{}, &fakeMailer{})
	assert.Error(t, err)
}

func TestCIP_UpdateUser(t *testing.T) {
	fake := &fakeAuthClient{
		getUser: func(_ context.Context, uid string) (*fbauth.UserRecord, error) {
			return &fbauth.UserRecord{
				UserInfo:      &fbauth.UserInfo{UID: uid, DisplayName: "New Name", Email: "new@example.com"},
				EmailVerified: true,
			}, nil
		},
	}
	a := newTestAuthenticator(fake)

	got, err := a.UpdateUser(context.Background(), gateway.AuthenticatorUpdateUserParam{
		ID:       "uid-1",
		Name:     strptr("New Name"),
		Email:    strptr("new@example.com"),
		Password: strptr("s3cret!!"),
	})
	assert.NoError(t, err)
	assert.Equal(t, gateway.AuthenticatorUser{
		ID:            "uid-1",
		Name:          "New Name",
		Email:         "new@example.com",
		EmailVerified: true,
	}, got)
	assert.Len(t, fake.updateCalls, 1)
	assert.Equal(t, "uid-1", fake.updateCalls[0].uid)
}

func TestCIP_UpdateUser_NothingToUpdate(t *testing.T) {
	a := newTestAuthenticator(&fakeAuthClient{})
	_, err := a.UpdateUser(context.Background(), gateway.AuthenticatorUpdateUserParam{ID: "uid-1"})
	assert.Error(t, err)
}

func TestCIP_UpdateUser_SDKError(t *testing.T) {
	fake := &fakeAuthClient{
		getUser: func(_ context.Context, _ string) (*fbauth.UserRecord, error) {
			return nil, errors.New("boom")
		},
	}
	a := newTestAuthenticator(fake)
	_, err := a.UpdateUser(context.Background(), gateway.AuthenticatorUpdateUserParam{
		ID:    "uid-1",
		Email: strptr("x@example.com"),
	})
	assert.Error(t, err)
}

func TestCIP_ResendVerificationEmail(t *testing.T) {
	fake := &fakeAuthClient{
		getUser: func(_ context.Context, uid string) (*fbauth.UserRecord, error) {
			return &fbauth.UserRecord{UserInfo: &fbauth.UserInfo{UID: uid, Email: "u@example.com"}}, nil
		},
	}
	fm := &fakeMailer{}
	a := &Authenticator{client: fake, mailer: fm}

	err := a.ResendVerificationEmail(context.Background(), "uid-1")
	assert.NoError(t, err)
	assert.Equal(t, "u@example.com", fake.verifyLinkEmail)
	// The generated link must actually be emailed (Admin SDK only generates it).
	assert.True(t, fm.sent)
	assert.Equal(t, "u@example.com", fm.to[0].Email)
	assert.Contains(t, fm.text, "https://verify.example/u@example.com")
	// The HTML body must embed the link in the href attribute.
	assert.Contains(t, fm.html, `href="https://verify.example/u@example.com"`)
}

func TestCIP_ResendVerificationEmail_HTMLEscapesLink(t *testing.T) {
	// Treat the verification URL as untrusted: any quote/angle bracket must be
	// escaped before being embedded into the href attribute of the HTML body.
	fake := &fakeAuthClient{
		getUser: func(_ context.Context, uid string) (*fbauth.UserRecord, error) {
			return &fbauth.UserRecord{UserInfo: &fbauth.UserInfo{UID: uid, Email: "u@example.com"}}, nil
		},
	}
	// Override EmailVerificationLink behavior via a wrapper so we can return a
	// link containing characters that must be escaped.
	wrapped := &linkOverrideClient{fakeAuthClient: fake, link: `https://verify.example/?token=a"b&c<d`}
	fm := &fakeMailer{}
	a := &Authenticator{client: wrapped, mailer: fm}

	err := a.ResendVerificationEmail(context.Background(), "uid-1")
	assert.NoError(t, err)
	assert.True(t, fm.sent)
	// Plain-text body keeps the raw link.
	assert.Contains(t, fm.text, `https://verify.example/?token=a"b&c<d`)
	// HTML body must contain the escaped form inside the href attribute and
	// must NOT contain the raw unescaped quote/ampersand/angle-bracket payload.
	assert.Contains(t, fm.html, `href="https://verify.example/?token=a&#34;b&amp;c&lt;d"`)
	assert.NotContains(t, fm.html, `a"b&c<d`)
}

// linkOverrideClient is a tiny wrapper around fakeAuthClient that returns a
// caller-supplied verification link, used to exercise HTML escaping.
type linkOverrideClient struct {
	*fakeAuthClient
	link string
}

func (l *linkOverrideClient) EmailVerificationLink(_ context.Context, email string) (string, error) {
	l.fakeAuthClient.verifyLinkEmail = email
	return l.link, nil
}

func TestCIP_ResendVerificationEmail_NoEmail(t *testing.T) {
	fake := &fakeAuthClient{
		getUser: func(_ context.Context, uid string) (*fbauth.UserRecord, error) {
			return &fbauth.UserRecord{UserInfo: &fbauth.UserInfo{UID: uid}}, nil
		},
	}
	a := newTestAuthenticator(fake)
	err := a.ResendVerificationEmail(context.Background(), "uid-1")
	assert.Error(t, err)
}
