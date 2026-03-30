package interactor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"golang.org/x/text/language"

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

func TestUser_UpdateMe(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	tests := []struct {
		name              string
		setupUser         func() (*user.User, *workspace.Workspace)
		setupExistingUser func() *user.User
		param             interfaces.UpdateMeParam
		wantErr           error
		verify            func(t *testing.T, r *repo.Container, u *user.User)
	}{
		{
			name: "update alias successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Alias("oldAlias").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Alias: strPtr("newAlias"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "newAlias", u.Alias())
			},
		},
		{
			name: "update alias fails when alias already exists",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Alias("myAlias").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			setupExistingUser: func() *user.User {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				return user.New().
					ID(uid).
					Workspace(wid).
					Name("Existing User").
					Alias("existingAlias").
					Email("existing@example.com").
					MustBuild()
			},
			param: interfaces.UpdateMeParam{
				Alias: strPtr("existingAlias"),
			},
			wantErr: interfaces.ErrUserAliasAlreadyExists,
			verify:  nil,
		},
		{
			name: "update alias with same alias does not error",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Alias("sameAlias").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Alias: strPtr("sameAlias"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "sameAlias", u.Alias())
			},
		},
		{
			name: "update description successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Description: strPtr("My new description"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "My new description", u.Metadata().Description())
				// Also verify workspace metadata is updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "My new description", ws.Metadata().Description())
			},
		},
		{
			name: "update website successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Website: strPtr("https://example.com"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				// Verify user metadata is updated
				assert.Equal(t, "https://example.com", u.Metadata().Website())
				// Verify workspace metadata is also updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "https://example.com", ws.Metadata().Website())
			},
		},
		{
			name: "update photoURL successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				PhotoURL: strPtr("https://example.com/photo.jpg"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				// Verify user metadata is updated
				assert.Equal(t, "https://example.com/photo.jpg", u.Metadata().PhotoURL())
				// Verify workspace metadata is also updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "https://example.com/photo.jpg", ws.Metadata().PhotoURL())
			},
		},
		{
			name: "update all metadata fields at once",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Description: strPtr("Full description"),
				Website:     strPtr("https://mysite.com"),
				PhotoURL:    strPtr("https://mysite.com/avatar.png"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				// Verify user metadata is updated
				assert.Equal(t, "Full description", u.Metadata().Description())
				assert.Equal(t, "https://mysite.com", u.Metadata().Website())
				assert.Equal(t, "https://mysite.com/avatar.png", u.Metadata().PhotoURL())
				// Verify workspace metadata is also updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "Full description", ws.Metadata().Description())
				assert.Equal(t, "https://mysite.com", ws.Metadata().Website())
				assert.Equal(t, "https://mysite.com/avatar.png", ws.Metadata().PhotoURL())
			},
		},
		{
			name: "update name and description together",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Old Name").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Old Name").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Name:        strPtr("New Name"),
				Description: strPtr("New description"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "New Name", u.Name())
				assert.Equal(t, "New description", u.Metadata().Description())
				// Verify workspace is also updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "New Name", ws.Name())
				assert.Equal(t, "New description", ws.Metadata().Description())
			},
		},
		{
			name: "workspace metadata not updated for non-personal workspace",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				// Non-personal workspace (Personal=false)
				w := workspace.New().
					ID(wid).
					Name("Team Workspace").
					Personal(false).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Description: strPtr("Should not update workspace"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				// User metadata should be updated
				assert.Equal(t, "Should not update workspace", u.Metadata().Description())
				// Workspace metadata should NOT be updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "", ws.Metadata().Description())
			},
		},
		{
			name: "update email successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("old@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Email: strPtr("new@example.com"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "new@example.com", u.Email())
			},
		},
		{
			name: "update email fails with invalid email",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Email: strPtr("invalid-email"),
			},
			wantErr: user.ErrInvalidEmail,
			verify:  nil,
		},
		{
			name: "update lang successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Lang: langPtr(language.Japanese),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, language.Japanese, u.Metadata().Lang())
			},
		},
		{
			name: "update theme successfully",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Theme: themePtr(user.ThemeDark),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, user.ThemeDark, u.Metadata().Theme())
			},
		},
		{
			name: "password update fails without confirmation",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					Auths([]user.Auth{{Provider: user.ProviderReearth, Sub: "reearth|" + uid.String()}}).
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Password: strPtr("NewPass123!"),
			},
			wantErr: interfaces.ErrUserInvalidPasswordConfirmation,
			verify:  nil,
		},
		{
			name: "password update fails with mismatched confirmation",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					Auths([]user.Auth{{Provider: user.ProviderReearth, Sub: "reearth|" + uid.String()}}).
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Password:             strPtr("NewPass123!"),
				PasswordConfirmation: strPtr("DifferentPass123!"),
			},
			wantErr: interfaces.ErrUserInvalidPasswordConfirmation,
			verify:  nil,
		},
		{
			name: "password update succeeds with matching confirmation",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Test User").
					Email("test@example.com").
					PasswordPlainText("OldPass123!").
					Auths([]user.Auth{{Provider: user.ProviderReearth, Sub: "reearth|" + uid.String()}}).
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Test User").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Password:             strPtr("NewPass123!"),
				PasswordConfirmation: strPtr("NewPass123!"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				matched, err := u.MatchPassword("NewPass123!")
				assert.NoError(t, err)
				assert.True(t, matched)
			},
		},
		{
			name: "workspace renamed when workspace name matches old user name",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Old Name").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Old Name"). // same as user name
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Name: strPtr("New Name"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "New Name", u.Name())
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "New Name", ws.Name())
			},
		},
		{
			name: "workspace NOT renamed when workspace name differs from old user name",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("User Name").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Custom Workspace Name"). // different from user name
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Name: strPtr("New User Name"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "New User Name", u.Name())
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				// Workspace name should remain unchanged
				assert.Equal(t, "Custom Workspace Name", ws.Name())
			},
		},
		{
			name: "workspace renamed when workspace name is empty",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Old Name").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name(""). // empty workspace name
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Name: strPtr("New Name"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "New Name", u.Name())
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "New Name", ws.Name())
			},
		},
		{
			name: "update multiple fields at once",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Old Name").
					Alias("oldAlias").
					Email("old@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Old Name").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Name:        strPtr("New Name"),
				Alias:       strPtr("newAlias"),
				Email:       strPtr("new@example.com"),
				Description: strPtr("New description"),
				Website:     strPtr("https://newsite.com"),
				Theme:       themePtr(user.ThemeLight),
				Lang:        langPtr(language.English),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "New Name", u.Name())
				assert.Equal(t, "newAlias", u.Alias())
				assert.Equal(t, "new@example.com", u.Email())
				assert.Equal(t, "New description", u.Metadata().Description())
				assert.Equal(t, user.ThemeLight, u.Metadata().Theme())
				assert.Equal(t, language.English, u.Metadata().Lang())

				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "New Name", ws.Name())
				assert.Equal(t, "New description", ws.Metadata().Description())
				assert.Equal(t, "https://newsite.com", ws.Metadata().Website())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := memory.New()
			uc := NewUser(r, nil, "", "")

			u, ws := tt.setupUser()
			assert.NoError(t, r.User.Save(ctx, u))
			assert.NoError(t, r.Workspace.Save(ctx, ws))

			if tt.setupExistingUser != nil {
				existingUser := tt.setupExistingUser()
				assert.NoError(t, r.User.Save(ctx, existingUser))
			}

			uid := u.ID()
			operator := &workspace.Operator{
				User: &uid,
			}

			result, err := uc.UpdateMe(ctx, tt.param, operator)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.verify != nil {
					tt.verify(t, r, result)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func langPtr(l language.Tag) *language.Tag {
	return &l
}

func themePtr(t user.Theme) *user.Theme {
	return &t
}

func TestUser_UpdateMe_NilOperatorUser(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	// Test with operator that has nil User
	operator := &workspace.Operator{
		User: nil,
	}
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_UserNotFound(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	// Create operator with non-existent user ID
	nonExistentUID := id.NewUserID()
	operator := &workspace.Operator{
		User: &nonExistentUID,
	}

	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.ErrorIs(t, err, rerror.ErrNotFound)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_FindByAliasError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Alias("oldAlias").
		Email("test@example.com").
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Test User").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	// Inject error into user repo for FindByAlias
	dbError := errors.New("database connection error")
	memory.SetUserError(r.User, dbError)

	operator := &workspace.Operator{
		User: &uid,
	}

	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Alias: strPtr("newAlias"),
	}, operator)

	assert.ErrorIs(t, err, dbError)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_SetPasswordError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Email("test@example.com").
		PasswordPlainText("OldPass123!").
		Auths([]user.Auth{{Provider: user.ProviderReearth, Sub: "reearth|" + uid.String()}}).
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Test User").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	operator := &workspace.Operator{
		User: &uid,
	}

	// Password too short should fail validation
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Password:             strPtr("short"),
		PasswordConfirmation: strPtr("short"),
	}, operator)

	assert.ErrorIs(t, err, user.ErrPasswordLength)
	assert.Nil(t, result)
}

// mockAuthenticatorWithError is a mock implementation of the Authenticator interface that returns errors
type mockAuthenticatorWithError struct {
	updateUserErr error
}

func (m *mockAuthenticatorWithError) UpdateUser(_ context.Context, _ gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	if m.updateUserErr != nil {
		return gateway.AuthenticatorUser{}, m.updateUserErr
	}
	return gateway.AuthenticatorUser{}, nil
}

func (m *mockAuthenticatorWithError) ResendVerificationEmail(_ context.Context, _ string) error {
	return nil
}

func TestUser_UpdateMe_AuthenticatorUpdateUserError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()

	authError := errors.New("auth0 api error")
	mockAuth := &mockAuthenticatorWithError{updateUserErr: authError}
	g := &gateway.Container{Authenticator: mockAuth}
	uc := NewUser(r, g, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Email("test@example.com").
		Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|123456"}}).
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Test User").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	operator := &workspace.Operator{
		User: &uid,
	}

	// This should trigger Auth0 update which will fail
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.ErrorIs(t, err, authError)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_WorkspaceSaveError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Old Name").
		Email("test@example.com").
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Old Name").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	// Inject error into workspace repo for Save
	dbError := errors.New("workspace save error")
	memory.SetWorkspaceError(r.Workspace, dbError)

	operator := &workspace.Operator{
		User: &uid,
	}

	// Name update triggers workspace rename and save
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.ErrorIs(t, err, dbError)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_UserSaveError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Email("test@example.com").
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Test User").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	// Inject error into user repo for Save
	dbError := errors.New("user save error")
	memory.SetUserError(r.User, dbError)

	operator := &workspace.Operator{
		User: &uid,
	}

	// This should fail on user save (email update doesn't trigger workspace save)
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Email: strPtr("new@example.com"),
	}, operator)

	assert.ErrorIs(t, err, dbError)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_WorkspaceFindByIDError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Old Name").
		Email("test@example.com").
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Old Name").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	// Inject error into workspace repo
	dbError := errors.New("workspace find error")
	memory.SetWorkspaceError(r.Workspace, dbError)

	operator := &workspace.Operator{
		User: &uid,
	}

	// Name update triggers workspace FindByID which will fail
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.ErrorIs(t, err, dbError)
	assert.Nil(t, result)
}

func TestUser_UpdateMe_WorkspaceMetadataFindByIDError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Email("test@example.com").
		MustBuild()
	ws := workspace.New().
		ID(wid).
		Name("Test User").
		Personal(true).
		MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	// Inject error into workspace repo
	dbError := errors.New("workspace find error for metadata")
	memory.SetWorkspaceError(r.Workspace, dbError)

	operator := &workspace.Operator{
		User: &uid,
	}

	// Description update triggers workspace metadata update path
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Description: strPtr("New description"),
	}, operator)

	assert.ErrorIs(t, err, dbError)
	assert.Nil(t, result)
}
