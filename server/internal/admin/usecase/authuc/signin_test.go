package authuc

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/admin/gateway/google"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeVerifier struct {
	claims *google.Claims
	err    error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (*google.Claims, error) {
	return f.claims, f.err
}

func newUC(t *testing.T, v google.Verifier, opts GoogleSignInOptions) (*GoogleSignInUseCase, adminuser.Repo) {
	t.Helper()
	repo := memory.NewAdminUser()
	return NewGoogleSignInUseCase(repo, v, opts), repo
}

func TestGoogleSignIn_NewUser_Pending(t *testing.T) {
	v := fakeVerifier{claims: &google.Claims{Email: "Alice@Eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Alice", PictureURL: "https://x/y.png"}}
	uc, repo := newUC(t, v, GoogleSignInOptions{AllowedDomain: "eukarya.io"})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, "alice@eukarya.io", u.Email())
	assert.Equal(t, "Alice", u.Name())
	assert.True(t, u.IsPending())
	assert.Equal(t, adminuser.Role(""), u.Role()) // non-bootstrap new account has no role

	// persisted
	got, err := repo.FindByEmail(context.Background(), "alice@eukarya.io")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())
}

func TestGoogleSignIn_NewUser_Bootstrapped(t *testing.T) {
	v := fakeVerifier{claims: &google.Claims{Email: "boss@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Boss"}}
	uc, _ := newUC(t, v, GoogleSignInOptions{AllowedDomain: "eukarya.io", BootstrapEmails: []string{"BOSS@eukarya.io"}})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.True(t, u.IsApproved())
	assert.False(t, u.ApprovedAt().IsZero())
	assert.Equal(t, adminuser.RoleSystemAdmin, u.Role())
}

func TestGoogleSignIn_ExistingUser_BootstrapElevatesAndApproves(t *testing.T) {
	// A pre-existing pending viewer whose email is (re-)added to the bootstrap
	// list is approved and elevated to system_admin on next sign-in.
	existing := adminuser.New().NewID().Email("boss@eukarya.io").Name("Boss").Status(adminuser.StatusPending).Role(adminuser.RoleViewer).MustBuild()
	repo := memory.NewAdminUserWith(existing)
	v := fakeVerifier{claims: &google.Claims{Email: "boss@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Boss"}}
	uc := NewGoogleSignInUseCase(repo, v, GoogleSignInOptions{AllowedDomain: "eukarya.io", BootstrapEmails: []string{"boss@eukarya.io"}})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, existing.ID(), u.ID())
	assert.True(t, u.IsApproved())
	assert.Equal(t, adminuser.RoleSystemAdmin, u.Role())

	got, err := repo.FindByEmail(context.Background(), "boss@eukarya.io")
	require.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.Equal(t, adminuser.RoleSystemAdmin, got.Role()) // persisted
}

func TestGoogleSignIn_ExistingUser_BootstrapSystemAdminNotDowngraded(t *testing.T) {
	existing := adminuser.New().NewID().Email("boss@eukarya.io").Name("Boss").Status(adminuser.StatusApproved).Role(adminuser.RoleSystemAdmin).MustBuild()
	repo := memory.NewAdminUserWith(existing)
	v := fakeVerifier{claims: &google.Claims{Email: "boss@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Boss"}}
	uc := NewGoogleSignInUseCase(repo, v, GoogleSignInOptions{AllowedDomain: "eukarya.io", BootstrapEmails: []string{"boss@eukarya.io"}})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.True(t, u.IsApproved())
	assert.Equal(t, adminuser.RoleSystemAdmin, u.Role())
}

func TestGoogleSignIn_ExistingUser_NonBootstrapRoleUntouched(t *testing.T) {
	existing := adminuser.New().NewID().Email("alice@eukarya.io").Name("Alice").Status(adminuser.StatusApproved).Role(adminuser.RoleViewer).MustBuild()
	repo := memory.NewAdminUserWith(existing)
	v := fakeVerifier{claims: &google.Claims{Email: "alice@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Alice"}}
	uc := NewGoogleSignInUseCase(repo, v, GoogleSignInOptions{AllowedDomain: "eukarya.io"})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.True(t, u.IsApproved())
	assert.Equal(t, adminuser.RoleViewer, u.Role()) // not bootstrapped: role unchanged
}

func TestGoogleSignIn_ExistingUser_RefreshesProfileAndKeepsStatus(t *testing.T) {
	existing := adminuser.New().NewID().Email("alice@eukarya.io").Name("Old").Status(adminuser.StatusApproved).MustBuild()
	repo := memory.NewAdminUserWith(existing)
	v := fakeVerifier{claims: &google.Claims{Email: "alice@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "New Name", PictureURL: "https://new"}}
	uc := NewGoogleSignInUseCase(repo, v, GoogleSignInOptions{AllowedDomain: "eukarya.io"})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, existing.ID(), u.ID())
	assert.Equal(t, "New Name", u.Name())
	assert.Equal(t, "https://new", u.PictureURL())
	assert.True(t, u.IsApproved()) // status unchanged
}

// raceRepo simulates a concurrent first sign-in: FindByEmail initially reports
// NotFound, Save fails with ErrDuplicatedAdminUser (another request won the
// race) but makes the record visible to the next FindByEmail.
type raceRepo struct {
	existing *adminuser.AdminUser
}

func (r *raceRepo) FindByEmail(_ context.Context, _ string) (*adminuser.AdminUser, error) {
	if r.existing != nil {
		return r.existing, nil
	}
	return nil, rerror.ErrNotFound
}
func (r *raceRepo) FindByID(context.Context, adminuser.ID) (*adminuser.AdminUser, error) {
	return nil, rerror.ErrNotFound
}
func (r *raceRepo) FindByIDs(context.Context, adminuser.IDList) (adminuser.List, error) {
	return nil, nil
}
func (r *raceRepo) List(context.Context, adminuser.ListFilter) (adminuser.List, *usecasex.PageInfo, error) {
	return nil, nil, nil
}
func (r *raceRepo) Save(_ context.Context, u *adminuser.AdminUser) error {
	r.existing = u
	return adminuser.ErrDuplicatedAdminUser
}

func TestGoogleSignIn_NewUser_DuplicateRaceReturnsExisting(t *testing.T) {
	repo := &raceRepo{}
	v := fakeVerifier{claims: &google.Claims{Email: "alice@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Alice"}}
	uc := NewGoogleSignInUseCase(repo, v, GoogleSignInOptions{AllowedDomain: "eukarya.io"})

	u, err := uc.Execute(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, "alice@eukarya.io", u.Email())
	assert.True(t, u.IsPending())
}

func TestGoogleSignIn_Errors(t *testing.T) {
	tests := []struct {
		name string
		v    fakeVerifier
		opts GoogleSignInOptions
		err  error
	}{
		{
			name: "invalid token",
			v:    fakeVerifier{err: errors.New("bad")},
			opts: GoogleSignInOptions{AllowedDomain: "eukarya.io"},
			err:  ErrInvalidToken,
		},
		{
			name: "email not verified",
			v:    fakeVerifier{claims: &google.Claims{Email: "a@eukarya.io", EmailVerified: false, HD: "eukarya.io"}},
			opts: GoogleSignInOptions{AllowedDomain: "eukarya.io"},
			err:  ErrEmailNotVerified,
		},
		{
			name: "wrong hd",
			v:    fakeVerifier{claims: &google.Claims{Email: "a@gmail.com", EmailVerified: true, HD: ""}},
			opts: GoogleSignInOptions{AllowedDomain: "eukarya.io"},
			err:  ErrDomainNotAllowed,
		},
		{
			name: "hd ok but email other domain",
			v:    fakeVerifier{claims: &google.Claims{Email: "a@evil.com", EmailVerified: true, HD: "eukarya.io"}},
			opts: GoogleSignInOptions{AllowedDomain: "eukarya.io"},
			err:  ErrDomainNotAllowed,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			uc, _ := newUC(t, tt.v, tt.opts)
			_, err := uc.Execute(context.Background(), "tok")
			assert.ErrorIs(t, err, tt.err)
		})
	}
}
