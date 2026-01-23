package interactor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"

	"github.com/stretchr/testify/assert"
)

func TestUser_VerifyUser(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	tests := []struct {
		name             string
		code             string
		createUserBefore func() *user.User
		wantUser         func(u *user.User, uid user.ID, tid user.WorkspaceID, expired time.Time) *user.User
		wantError        error
	}{
		{
			name: "ok",
			code: "code",
			createUserBefore: func() *user.User {
				uid := id.NewUserID()
				tid := id.NewWorkspaceID()
				expired := time.Now().Add(24 * time.Hour)
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom("code", expired, false)).
					MustBuild()
			},
			wantUser: func(u *user.User, uid user.ID, tid user.WorkspaceID, expired time.Time) *user.User {
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom("code", expired, true)).
					MustBuild()
			},
			wantError: nil,
		},
		{
			name: "expired",
			code: "code",
			createUserBefore: func() *user.User {
				uid := id.NewUserID()
				tid := id.NewWorkspaceID()
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom("code", time.Now().Add(-24*time.Hour), false)).
					MustBuild()
			},
			wantUser:  nil,
			wantError: errors.New("verification expired"),
		},
		{
			name: "not found",
			code: "codesss",
			createUserBefore: func() *user.User {
				uid := id.NewUserID()
				tid := id.NewWorkspaceID()
				expired := time.Now().Add(24 * time.Hour)
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom("code", expired, false)).
					MustBuild()
			},
			wantUser:  nil,
			wantError: rerror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			// Create a new repository instance for each subtest to avoid race conditions
			r := memory.New()
			uc := NewUser(r, nil, "", "")

			var createdUser *user.User
			if tt.createUserBefore != nil {
				createdUser = tt.createUserBefore()
				assert.NoError(t, r.User.Save(ctx, createdUser))
			}

			u, err := uc.VerifyUser(ctx, tt.code)

			if tt.wantUser != nil && createdUser != nil {
				expired := createdUser.Verification().Expiration()
				expectedUser := tt.wantUser(u, createdUser.ID(), createdUser.Workspace(), expired)

				// Compare fields except updatedAt which is set dynamically
				assert.Equal(t, expectedUser.ID(), u.ID())
				assert.Equal(t, expectedUser.Name(), u.Name())
				assert.Equal(t, expectedUser.Email(), u.Email())
				assert.Equal(t, expectedUser.Workspace(), u.Workspace())
				assert.Equal(t, expectedUser.Verification().IsVerified(), u.Verification().IsVerified())
			} else {
				assert.Nil(t, u)
			}
			assert.Equal(t, tt.wantError, err)
		})
	}
}

func TestUser_StartPasswordReset(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	uid := id.NewUserID()
	tid := id.NewWorkspaceID()
	r := memory.New()

	m := mailer.NewMock()
	g := &gateway.Container{Mailer: m}
	uc := NewUser(r, g, "", "")
	tests := []struct {
		name             string
		createUserBefore *user.User
		email            string
		wantMailSubject  string
		wantMailTo       []mailer.Contact
		wantError        error
	}{
		{
			name: "ok",
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Email("aaa@bbb.com").
				Name("NAME").
				Auths([]user.Auth{
					{
						Provider: user.ProviderReearth,
						Sub:      "reearth|" + uid.String(),
					},
				}).
				MustBuild(),
			email:           "aaa@bbb.com",
			wantMailSubject: "Password reset",
			wantMailTo: []mailer.Contact{
				{
					Email: "aaa@bbb.com",
					Name:  "NAME",
				},
			},
			wantError: nil,
		},
		{
			name:      "not found",
			email:     "ccc@bbb.com",
			wantError: rerror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.createUserBefore != nil {
				assert.NoError(t, r.User.Save(ctx, tt.createUserBefore))
			}
			err := uc.StartPasswordReset(ctx, tt.email)

			if err != nil {
				assert.Equal(t, tt.wantError, err)
			} else {
				user, err := r.User.FindByEmail(ctx, tt.email)
				assert.NoError(t, err)
				assert.NotNil(t, user.PasswordReset())
			}

			mails := m.Mails()
			if tt.wantMailSubject != "" {
				assert.Equal(t, 1, len(mails))
				assert.Equal(t, tt.wantMailSubject, mails[0].Subject)
				assert.Equal(t, tt.wantMailTo, mails[0].To)
			}
		})
	}
}

func TestUser_PasswordReset(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	uid := id.NewUserID()
	tid := id.NewWorkspaceID()
	r := memory.New()
	uc := NewUser(r, nil, "", "")
	pr := user.NewPasswordReset()
	expired := time.Now().Add(24 * time.Hour)
	tests := []struct {
		name             string
		password         string
		token            string
		createUserBefore *user.User
		wantError        error
	}{
		{
			name:     "ok",
			password: "PAss00!!",
			token:    pr.Token,
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Email("aaa@bbb.com").
				PasswordPlainText("PAss00!!").
				Verification(user.VerificationFrom("code", expired, false)).
				PasswordReset(pr).
				Auths([]user.Auth{
					{
						Provider: user.ProviderReearth,
						Sub:      "reearth|" + uid.String(),
					},
				}).
				MustBuild(),
			wantError: nil,
		},
		{
			name:     "invalid password",
			password: "pass",
			token:    pr.Token,
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Email("aaa@bbb.com").
				PasswordPlainText("PAss00!!").
				Verification(user.VerificationFrom("code", expired, false)).
				PasswordReset(pr).
				Auths([]user.Auth{
					{
						Provider: user.ProviderReearth,
						Sub:      "reearth|" + uid.String(),
					},
				}).
				MustBuild(),
			wantError: user.ErrPasswordLength,
		},
		{
			name:     "not found",
			password: "PAss00!!",
			token:    pr.Token,
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Email("aaa@bbb.com").
				PasswordPlainText("PAss00!!").
				Verification(user.VerificationFrom("code", expired, false)).
				Auths([]user.Auth{
					{
						Provider: user.ProviderReearth,
						Sub:      "reearth|" + uid.String(),
					},
				}).
				MustBuild(),
			wantError: rerror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.createUserBefore != nil {
				assert.NoError(t, r.User.Save(ctx, tt.createUserBefore))
			}
			err := uc.PasswordReset(ctx, tt.password, tt.token)
			assert.Equal(t, tt.wantError, err)
		})
	}
}
