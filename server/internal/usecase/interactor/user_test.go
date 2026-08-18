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
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
	"golang.org/x/text/language"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func maintainerOperator(ctx context.Context, t *testing.T, db *repo.Container) *workspace.Operator {
	t.Helper()

	uid := user.NewID()
	maintainerRole := role.New().NewID().Name(role.RoleMaintainer.String()).MustBuild()
	require.NoError(t, db.Role.Save(ctx, *maintainerRole))

	p := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs(id.RoleIDList{maintainerRole.ID()}).
		MustBuild()
	require.NoError(t, db.Permittable.Save(ctx, *p))

	return &workspace.Operator{User: lo.ToPtr(uid)}
}

// ownerOperator returns an operator holding the global Permittable "owner" role
// (e.g. LINKS-Veda's admin account), distinct from a per-workspace owner role.
func ownerOperator(ctx context.Context, t *testing.T, db *repo.Container) *workspace.Operator {
	t.Helper()

	uid := user.NewID()
	ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
	require.NoError(t, db.Role.Save(ctx, *ownerRole))

	p := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs(id.RoleIDList{ownerRole.ID()}).
		MustBuild()
	require.NoError(t, db.Permittable.Save(ctx, *p))

	return &workspace.Operator{User: lo.ToPtr(uid)}
}

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
			uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, g, nil, "", "")
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
		{
			name: "no reearth auth",
			createUserBefore: user.New().
				ID(id.NewUserID()).
				Workspace(id.NewWorkspaceID()).
				Email("noauth@bbb.com").
				Name("NOAUTH").
				Auths([]user.Auth{
					{
						Provider: "auth0",
						Sub:      "auth0|someuser",
					},
				}).
				MustBuild(),
			email:     "noauth@bbb.com",
			wantError: interfaces.ErrUserInvalidPasswordReset,
		},
		{
			name: "empty sub",
			createUserBefore: user.New().
				ID(id.NewUserID()).
				Workspace(id.NewWorkspaceID()).
				Email("emptysub@bbb.com").
				Name("EMPTYSUB").
				Auths([]user.Auth{
					{
						Provider: user.ProviderReearth,
						Sub:      "",
					},
				}).
				MustBuild(),
			email:     "emptysub@bbb.com",
			wantError: interfaces.ErrUserInvalidPasswordReset,
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
	uc := NewUser(r, nil, nil, "", "")
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

func TestUser_Logout(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		r := memory.New()
		uc := NewUser(r, nil, nil, "", "")

		uid := id.NewUserID()
		tid := id.NewWorkspaceID()
		u := user.New().
			ID(uid).
			Workspace(tid).
			Name("NAME").
			Email("aaa@bbb.com").
			MustBuild()
		assert.NoError(t, r.User.Save(ctx, u))

		before := time.Now()
		op := &workspace.Operator{User: lo.ToPtr(uid)}
		result, err := uc.Logout(ctx, op)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.LatestLogoutAt().IsZero())
		assert.True(t, !result.LatestLogoutAt().Before(before))

		// Verify persisted
		saved, err := r.User.FindByID(ctx, uid)
		assert.NoError(t, err)
		assert.Equal(t, result.LatestLogoutAt().Unix(), saved.LatestLogoutAt().Unix())
	})

	t.Run("nil operator user", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		r := memory.New()
		uc := NewUser(r, nil, nil, "", "")

		op := &workspace.Operator{}
		result, err := uc.Logout(ctx, op)

		assert.Nil(t, result)
		assert.Equal(t, interfaces.ErrInvalidOperator, err)
	})
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
					Alias("oldAlias").
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
				// Verify workspace alias is also updated
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "newAlias", ws.Alias())
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
					Alias("sameAlias").
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
				// Verify workspace alias remains unchanged
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "sameAlias", ws.Alias())
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
			name: "update alias and name together updates workspace alias",
			setupUser: func() (*user.User, *workspace.Workspace) {
				uid := id.NewUserID()
				wid := id.NewWorkspaceID()
				u := user.New().
					ID(uid).
					Workspace(wid).
					Name("Old Name").
					Alias("oldAlias").
					Email("test@example.com").
					MustBuild()
				w := workspace.New().
					ID(wid).
					Name("Different Workspace Name").
					Alias("oldAlias").
					Personal(true).
					MustBuild()
				return u, w
			},
			param: interfaces.UpdateMeParam{
				Alias: strPtr("newAlias"),
				Name:  strPtr("New Name"),
			},
			wantErr: nil,
			verify: func(t *testing.T, r *repo.Container, u *user.User) {
				assert.Equal(t, "newAlias", u.Alias())
				assert.Equal(t, "New Name", u.Name())
				// Verify workspace alias is updated even when workspace name differs from old user name
				ws, err := r.Workspace.FindByID(context.Background(), u.Workspace())
				assert.NoError(t, err)
				assert.Equal(t, "newAlias", ws.Alias())
				// Workspace name should NOT be updated since it differs from old user name
				assert.Equal(t, "Different Workspace Name", ws.Name())
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
			uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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

// mockAuthenticatorWithError is a mock implementation of the Authenticator interface.
// All methods succeed by default; set updateUserErr to make UpdateUser fail.
type mockAuthenticatorWithError struct {
	updateUserErr error
}

func (m *mockAuthenticatorWithError) UpdateUser(_ context.Context, _ gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	if m.updateUserErr != nil {
		return gateway.AuthenticatorUser{}, m.updateUserErr
	}
	return gateway.AuthenticatorUser{}, nil
}

func (m *mockAuthenticatorWithError) DisableMFA(_ context.Context, _ string) error { return nil }

func (m *mockAuthenticatorWithError) EnableMFA(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockAuthenticatorWithError) GetMFAStatus(_ context.Context, _ string) (gateway.MFAStatus, error) {
	return gateway.MFAStatus{}, nil
}

func (m *mockAuthenticatorWithError) RegenerateMFARecoveryCode(_ context.Context, _ string) (string, error) {
	return "new-recovery-code", nil
}

func (m *mockAuthenticatorWithError) ResendVerificationEmail(_ context.Context, _ string) error {
	return nil
}

func TestUser_RegenerateMFARecoveryCode(t *testing.T) {
	ctx := context.Background()

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

	newUC := func() (interfaces.User, *workspace.Operator) {
		r := memory.New()
		assert.NoError(t, r.User.Save(ctx, u))
		assert.NoError(t, r.Workspace.Save(ctx, ws))

		mockAuth := &mockAuthenticatorWithError{}
		g := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderAuth0: mockAuth}}

		return NewUser(r, g, nil, "", ""), &workspace.Operator{User: &uid}
	}

	t.Run("ok", func(t *testing.T) {
		uc, operator := newUC()
		code, err := uc.RegenerateMFARecoveryCode(ctx, operator, "")
		assert.NoError(t, err)
		assert.Equal(t, "new-recovery-code", code)
	})

	t.Run("nil operator", func(t *testing.T) {
		uc, _ := newUC()
		code, err := uc.RegenerateMFARecoveryCode(ctx, nil, "")
		assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
		assert.Empty(t, code)
	})

	t.Run("no auth0 auth record", func(t *testing.T) {
		r := memory.New()
		noAuthUID := id.NewUserID()
		noAuthWID := id.NewWorkspaceID()
		noAuthUser := user.New().
			ID(noAuthUID).
			Workspace(noAuthWID).
			Name("No Auth User").
			Email("noauth@example.com").
			MustBuild()
		noAuthWs := workspace.New().
			ID(noAuthWID).
			Name("No Auth User").
			Personal(true).
			MustBuild()
		assert.NoError(t, r.User.Save(ctx, noAuthUser))
		assert.NoError(t, r.Workspace.Save(ctx, noAuthWs))

		g := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{}}
		uc := NewUser(r, g, nil, "", "")

		code, err := uc.RegenerateMFARecoveryCode(ctx, &workspace.Operator{User: &noAuthUID}, "")
		assert.Error(t, err)
		assert.Empty(t, code)
	})

	newPasswordUC := func() (interfaces.User, *workspace.Operator, id.UserID) {
		pwUID := id.NewUserID()
		pwWID := id.NewWorkspaceID()
		pwUser := user.New().
			ID(pwUID).
			Workspace(pwWID).
			Name("Password User").
			Email("password-user@example.com").
			PasswordPlainText("CurrentPass123!").
			Auths([]user.Auth{
				{Provider: user.ProviderReearth, Sub: "reearth|" + pwUID.String()},
				{Provider: "auth0", Sub: "auth0|password-user"},
			}).
			MustBuild()
		pwWs := workspace.New().
			ID(pwWID).
			Name("Password User").
			Personal(true).
			MustBuild()

		r := memory.New()
		assert.NoError(t, r.User.Save(ctx, pwUser))
		assert.NoError(t, r.Workspace.Save(ctx, pwWs))

		mockAuth := &mockAuthenticatorWithError{}
		g := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderAuth0: mockAuth}}

		return NewUser(r, g, nil, "", ""), &workspace.Operator{User: &pwUID}, pwUID
	}

	t.Run("password account requires current password", func(t *testing.T) {
		uc, operator, _ := newPasswordUC()
		code, err := uc.RegenerateMFARecoveryCode(ctx, operator, "")
		assert.ErrorIs(t, err, interfaces.ErrInvalidCurrentPassword)
		assert.Empty(t, code)
	})

	t.Run("password account rejects wrong current password", func(t *testing.T) {
		uc, operator, _ := newPasswordUC()
		code, err := uc.RegenerateMFARecoveryCode(ctx, operator, "WrongPass123!")
		assert.ErrorIs(t, err, interfaces.ErrInvalidCurrentPassword)
		assert.Empty(t, code)
	})

	t.Run("password account succeeds with correct current password", func(t *testing.T) {
		uc, operator, _ := newPasswordUC()
		code, err := uc.RegenerateMFARecoveryCode(ctx, operator, "CurrentPass123!")
		assert.NoError(t, err)
		assert.Equal(t, "new-recovery-code", code)
	})
}

func TestUser_UpdateMe_AuthenticatorUpdateUserError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()

	authError := errors.New("auth0 api error")
	mockAuth := &mockAuthenticatorWithError{updateUserErr: authError}
	g := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderAuth0: mockAuth}}
	uc := NewUser(r, g, nil, "", "")

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

func TestUser_UpdateMe_SkipsIdPSyncForCIP(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()

	mockAuth := &mockAuthenticatorWithError{updateUserErr: errors.New("cip should not be called")}
	g := &gateway.Container{Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderCIP: mockAuth}}
	uc := NewUser(r, g, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(wid).
		Name("Test User").
		Email("test@example.com").
		Auths([]user.Auth{{Provider: "", Sub: "cip-sub-123"}}).
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

	// CIP auth records are skipped entirely, so the mock's error is never hit.
	result, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{
		Name: strPtr("New Name"),
	}, operator)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Name", result.Name())
}

func TestUser_UpdateMe_WorkspaceSaveError(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}

	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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
	uc := NewUser(r, nil, nil, "", "")

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

func TestUser_DeleteMe_DeletesUserAndPersonalWorkspace(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u := user.New().ID(uid).Workspace(wid).Name("Test").Email("test@example.com").MustBuild()
	ws := workspace.New().ID(wid).Name("Test").Personal(true).Members(map[workspace.UserID]workspace.Member{
		uid: {Role: role.RoleOwner},
	}).MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, ws))

	op := &workspace.Operator{User: &uid}
	assert.NoError(t, uc.DeleteMe(ctx, uid, op))

	_, err := r.User.FindByID(ctx, uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	_, err = r.Workspace.FindByID(ctx, wid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func TestUser_DeleteMe_LeavesSharedWorkspaceAndDeletesUser(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	sharedWID := id.NewWorkspaceID()
	ownerUID := id.NewUserID()

	u := user.New().ID(uid).Workspace(wid).Name("Test").Email("test@example.com").MustBuild()
	personalWS := workspace.New().ID(wid).Name("Test").Personal(true).Members(map[workspace.UserID]workspace.Member{
		uid: {Role: role.RoleOwner},
	}).MustBuild()
	sharedWS := workspace.New().ID(sharedWID).Name("Shared").Members(map[workspace.UserID]workspace.Member{
		uid:      {Role: role.RoleWriter},
		ownerUID: {Role: role.RoleOwner},
	}).MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, personalWS))
	assert.NoError(t, r.Workspace.Save(ctx, sharedWS))

	op := &workspace.Operator{User: &uid}
	assert.NoError(t, uc.DeleteMe(ctx, uid, op))

	// User and personal workspace deleted
	_, err := r.User.FindByID(ctx, uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	_, err = r.Workspace.FindByID(ctx, wid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	// Shared workspace persists without the deleted user
	remaining, err := r.Workspace.FindByID(ctx, sharedWID)
	assert.NoError(t, err)
	assert.False(t, remaining.Members().HasUser(uid))
	assert.True(t, remaining.Members().HasUser(ownerUID))
}

func TestUser_DeleteMe_SoleOwnerOfSharedWorkspaceDeleted(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	uc := NewUser(r, nil, nil, "", "")

	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	ownedWID := id.NewWorkspaceID()

	u := user.New().ID(uid).Workspace(wid).Name("Test").Email("test@example.com").MustBuild()
	personalWS := workspace.New().ID(wid).Name("Test").Personal(true).Members(map[workspace.UserID]workspace.Member{
		uid: {Role: role.RoleOwner},
	}).MustBuild()
	// Non-personal workspace where the user is the sole owner
	ownedWS := workspace.New().ID(ownedWID).Name("Owned").Members(map[workspace.UserID]workspace.Member{
		uid: {Role: role.RoleOwner},
	}).MustBuild()

	assert.NoError(t, r.User.Save(ctx, u))
	assert.NoError(t, r.Workspace.Save(ctx, personalWS))
	assert.NoError(t, r.Workspace.Save(ctx, ownedWS))

	op := &workspace.Operator{User: &uid}
	assert.NoError(t, uc.DeleteMe(ctx, uid, op))

	_, err := r.User.FindByID(ctx, uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	// Both workspaces deleted since user was sole owner of both
	_, err = r.Workspace.FindByID(ctx, wid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	_, err = r.Workspace.FindByID(ctx, ownedWID)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

type failingMailer struct{ err error }

func (f *failingMailer) SendMail(_ context.Context, _ []mailer.Contact, _, _, _ string) error {
	return f.err
}

func TestUser_StartPasswordReset_TokenPersistedBeforeMailSend(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	t.Parallel()

	ctx := context.Background()
	r := memory.New()
	mailerErr := errors.New("smtp unavailable")
	g := &gateway.Container{Mailer: &failingMailer{err: mailerErr}}
	uc := NewUser(r, g, nil, "", "")

	uid := id.NewUserID()
	tid := id.NewWorkspaceID()
	u := user.New().
		ID(uid).
		Workspace(tid).
		Email("reset@bbb.com").
		Name("RESET").
		Auths([]user.Auth{
			{Provider: user.ProviderReearth, Sub: "reearth|" + uid.String()},
		}).
		MustBuild()
	assert.NoError(t, r.User.Save(ctx, u))

	err := uc.StartPasswordReset(ctx, "reset@bbb.com")

	assert.ErrorIs(t, err, mailerErr)

	// Token must be committed to DB even though the mailer failed.
	// This is the key invariant of the fix: the transaction commits before SendMail is called,
	// so a mailer failure never produces a sent email with an invalidated token.
	saved, dbErr := r.User.FindByEmail(ctx, "reset@bbb.com")
	assert.NoError(t, dbErr)
	assert.NotNil(t, saved.PasswordReset(), "token must be persisted even when mailer fails")
}

func TestUser_FindAll(t *testing.T) {
	ctx := context.Background()
	db := memory.New()
	op := maintainerOperator(ctx, t, db)
	userUC := NewUser(db, nil, nil, "", "")

	uA := user.New().NewID().Name("alpha").Email("alpha@bbb.com").MustBuild()
	uB := user.New().NewID().Name("beta").Email("beta@bbb.com").MustBuild()
	assert.NoError(t, db.User.Save(ctx, uA))
	assert.NoError(t, db.User.Save(ctx, uB))

	t.Run("default status excludes soft-deleted users", func(t *testing.T) {
		uA.Deactivate()
		defer uA.Reactivate()
		assert.NoError(t, db.User.Save(ctx, uA))

		res, err := userUC.FindAll(ctx, interfaces.FindAllUsersParam{Page: 1, Size: 10, Operator: op})
		assert.NoError(t, err)
		assert.Len(t, res.Users, 1)
		assert.Equal(t, uB.ID(), res.Users[0].ID())

		res, err = userUC.FindAll(ctx, interfaces.FindAllUsersParam{Status: user.StatusDeleted, Page: 1, Size: 10, Operator: op})
		assert.NoError(t, err)
		assert.Len(t, res.Users, 1)
		assert.Equal(t, uA.ID(), res.Users[0].ID())

		res, err = userUC.FindAll(ctx, interfaces.FindAllUsersParam{Status: user.StatusAll, Page: 1, Size: 10, Operator: op})
		assert.NoError(t, err)
		assert.Len(t, res.Users, 2)
	})

	t.Run("keyword filters by name", func(t *testing.T) {
		kw := "alpha"
		res, err := userUC.FindAll(ctx, interfaces.FindAllUsersParam{Keyword: &kw, Status: user.StatusAll, Page: 1, Size: 10, Operator: op})
		assert.NoError(t, err)
		assert.Len(t, res.Users, 1)
		assert.Equal(t, uA.ID(), res.Users[0].ID())
	})

	t.Run("global owner role (e.g. LINKS-Veda's admin account) can also list", func(t *testing.T) {
		ownerOp := ownerOperator(ctx, t, db)
		res, err := userUC.FindAll(ctx, interfaces.FindAllUsersParam{Status: user.StatusAll, Page: 1, Size: 10, Operator: ownerOp})
		assert.NoError(t, err)
		assert.Len(t, res.Users, 2)
	})

	t.Run("denies a nil operator", func(t *testing.T) {
		_, err := userUC.FindAll(ctx, interfaces.FindAllUsersParam{Page: 1, Size: 10})
		assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
	})

	t.Run("denies an operator without the maintainer role", func(t *testing.T) {
		nonMaintainer := user.NewID()
		p := permittable.New().NewID().UserID(nonMaintainer).MustBuild()
		assert.NoError(t, db.Permittable.Save(ctx, *p))
		nonMaintainerOp := &workspace.Operator{User: lo.ToPtr(nonMaintainer)}

		_, err := userUC.FindAll(ctx, interfaces.FindAllUsersParam{Page: 1, Size: 10, Operator: nonMaintainerOp})
		assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)
	})
}

func TestUser_UpdateUserBySub(t *testing.T) {
	ctx := context.Background()

	t.Run("maintainer can update name by sub", func(t *testing.T) {
		db := memory.New()
		op := maintainerOperator(ctx, t, db)

		wid := id.NewWorkspaceID()
		u := user.New().NewID().Workspace(wid).Name("Old Name").Email("bysub@bbb.com").
			Auths([]user.Auth{{Provider: "", Sub: "cip-sub-1"}}).MustBuild()
		ws := workspace.New().ID(wid).Name("Old Name").Personal(true).MustBuild()
		assert.NoError(t, db.User.Save(ctx, u))
		assert.NoError(t, db.Workspace.Save(ctx, ws))

		uc := NewUser(db, nil, nil, "", "")
		assert.NoError(t, uc.UpdateUserBySub(ctx, "cip-sub-1", strPtr("New Name"), op))

		got, err := db.User.FindBySub(ctx, "cip-sub-1")
		assert.NoError(t, err)
		assert.Equal(t, "New Name", got.Name())
	})

	t.Run("denies a nil operator", func(t *testing.T) {
		db := memory.New()
		uc := NewUser(db, nil, nil, "", "")
		err := uc.UpdateUserBySub(ctx, "cip-sub-1", strPtr("New Name"), nil)
		assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
	})

	t.Run("denies an operator without the maintainer role", func(t *testing.T) {
		db := memory.New()
		nonMaintainer := user.NewID()
		p := permittable.New().NewID().UserID(nonMaintainer).MustBuild()
		assert.NoError(t, db.Permittable.Save(ctx, *p))
		op := &workspace.Operator{User: lo.ToPtr(nonMaintainer)}

		uc := NewUser(db, nil, nil, "", "")
		err := uc.UpdateUserBySub(ctx, "cip-sub-1", strPtr("New Name"), op)
		assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)
	})
}

func TestUser_SetPlatformRolesBySub(t *testing.T) {
	ctx := context.Background()

	t.Run("maintainer can replace platform roles by sub", func(t *testing.T) {
		db := memory.New()
		op := maintainerOperator(ctx, t, db)

		targetRole := role.New().NewID().Name("custom").MustBuild()
		assert.NoError(t, db.Role.Save(ctx, *targetRole))

		u := user.New().NewID().Workspace(id.NewWorkspaceID()).Name("Sub User").Email("sub2@bbb.com").
			Auths([]user.Auth{{Provider: "", Sub: "cip-sub-2"}}).MustBuild()
		assert.NoError(t, db.User.Save(ctx, u))

		uc := NewUser(db, nil, nil, "", "")
		assert.NoError(t, uc.SetPlatformRolesBySub(ctx, "cip-sub-2", []string{"custom"}, op))

		p, err := db.Permittable.FindByUserID(ctx, u.ID())
		assert.NoError(t, err)
		assert.Contains(t, p.RoleIDs(), targetRole.ID())
	})

	t.Run("denies an operator without the maintainer role", func(t *testing.T) {
		db := memory.New()
		nonMaintainer := user.NewID()
		p := permittable.New().NewID().UserID(nonMaintainer).MustBuild()
		assert.NoError(t, db.Permittable.Save(ctx, *p))
		op := &workspace.Operator{User: lo.ToPtr(nonMaintainer)}

		uc := NewUser(db, nil, nil, "", "")
		err := uc.SetPlatformRolesBySub(ctx, "cip-sub-2", []string{"custom"}, op)
		assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)
	})
}

func TestUser_DeactivateAndRestore(t *testing.T) {
	ctx := context.Background()

	newTargetUser := func() (user.ID, *repo.Container) {
		db := memory.New()
		uid := id.NewUserID()
		u := user.New().ID(uid).Workspace(id.NewWorkspaceID()).Name("Target").Email("target@bbb.com").MustBuild()
		assert.NoError(t, db.User.Save(ctx, u))
		return uid, db
	}

	t.Run("maintainer can deactivate then restore", func(t *testing.T) {
		uid, db := newTargetUser()
		op := maintainerOperator(ctx, t, db)
		uc := NewUser(db, nil, nil, "", "")

		u, err := uc.Deactivate(ctx, uid, op)
		assert.NoError(t, err)
		assert.NotNil(t, u.DeletedAt())

		stored, err := db.User.FindByID(ctx, uid)
		assert.NoError(t, err)
		assert.NotNil(t, stored.DeletedAt())

		u, err = uc.Restore(ctx, uid, op)
		assert.NoError(t, err)
		assert.Nil(t, u.DeletedAt())

		stored, err = db.User.FindByID(ctx, uid)
		assert.NoError(t, err)
		assert.Nil(t, stored.DeletedAt())
	})

	t.Run("denies an operator without the maintainer role", func(t *testing.T) {
		uid, db := newTargetUser()
		nonMaintainer := user.NewID()
		p := permittable.New().NewID().UserID(nonMaintainer).MustBuild()
		assert.NoError(t, db.Permittable.Save(ctx, *p))
		op := &workspace.Operator{User: lo.ToPtr(nonMaintainer)}
		uc := NewUser(db, nil, nil, "", "")

		_, err := uc.Deactivate(ctx, uid, op)
		assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)

		_, err = uc.Restore(ctx, uid, op)
		assert.ErrorIs(t, err, interfaces.ErrPermissionDenied)
	})

	t.Run("denies a nil operator", func(t *testing.T) {
		uid, db := newTargetUser()
		uc := NewUser(db, nil, nil, "", "")

		_, err := uc.Deactivate(ctx, uid, &workspace.Operator{})
		assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)

		_, err = uc.Restore(ctx, uid, &workspace.Operator{})
		assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
	})
}
