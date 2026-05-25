package cip

import (
	"context"
	"errors"
	"testing"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/stretchr/testify/assert"
)

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
	return &Authenticator{client: c}
}

func strptr(s string) *string { return &s }

func TestCIP_New_RequiresProjectID(t *testing.T) {
	_, err := New(context.Background(), Params{})
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
	a := newTestAuthenticator(fake)

	err := a.ResendVerificationEmail(context.Background(), "uid-1")
	assert.NoError(t, err)
	assert.Equal(t, "u@example.com", fake.verifyLinkEmail)
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
