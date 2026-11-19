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
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type signupFixture struct {
	clock  time.Time
	policy *Policy
	repos  *repo.Container
	users  interfaces.User
}

func newSignupFixture(t *testing.T, secret string) *signupFixture {
	t.Helper()
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	ctx := context.Background()
	r := memory.New()
	for _, name := range []string{interfaces.RoleSelf, role.RoleOwner.String()} {
		require.NoError(t, r.Role.Save(ctx, *role.New().NewID().Name(name).MustBuild()))
	}
	f := &signupFixture{clock: time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC), repos: r}
	cat := NewPolicyCatalog(r.PolicyVersion, PolicyCatalogTTL)
	cat.now = func() time.Time { return f.clock }
	f.policy = NewPolicy(r, legal.NewSecret(secret), cat)
	f.policy.now = func() time.Time { return f.clock }
	f.users = NewUser(r, &gateway.Container{}, nil, "", "", WithPolicy(f.policy))
	return f
}

func (f *signupFixture) publishTos(t *testing.T, version string, effective time.Time) {
	t.Helper()
	f.clock = f.clock.Add(time.Second)
	m := legal.Manifest{Timestamp: f.clock, Entries: []legal.Entry{{
		Ref:         tosRef(version),
		EffectiveAt: effective,
		Required:    true,
		Title:       legal.LocalizedText{"en": "ToS"},
	}}}
	_, err := legal.Apply(context.Background(), f.repos.PolicyVersion, m, func() time.Time { return f.clock })
	require.NoError(t, err)
}

func (f *signupFixture) signup(policies ...legal.RefInput) (*user.User, error) {
	return f.users.Signup(context.Background(), interfaces.SignupParam{
		Client:   testClient,
		Email:    "new@example.com",
		MockAuth: true,
		Name:     "new",
		Password: "PAss00!!",
		Policies: policies,
	})
}

func (f *signupFixture) signupOIDC(policies ...legal.RefInput) (*user.User, error) {
	return f.users.SignupOIDC(context.Background(), interfaces.SignupOIDCParam{
		Client:   testClient,
		Email:    "new@example.com",
		Name:     "new",
		Policies: policies,
		Sub:      "auth0|new",
	})
}

func (f *signupFixture) pending(t *testing.T, uid user.ID) []legal.Ref {
	t.Helper()
	ov, err := f.policy.Overview(context.Background(), &workspace.Operator{User: &uid})
	require.NoError(t, err)
	res := []legal.Ref{}
	for _, v := range ov.Pending {
		res = append(res, v.Ref)
	}
	return res
}

func (f *signupFixture) assertNoUser(t *testing.T) {
	t.Helper()
	_, err := f.repos.User.FindByEmail(context.Background(), "new@example.com")
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func (f *signupFixture) records(t *testing.T, uid user.ID) []legal.Acceptance {
	t.Helper()
	as, err := f.repos.PolicyAcceptance.FindByUser(context.Background(), uid)
	require.NoError(t, err)
	return as
}

func TestUser_SignupRecordsPolicies(t *testing.T) {
	for name, signup := range map[string]func(*signupFixture, ...legal.RefInput) (*user.User, error){
		"signup":     (*signupFixture).signup,
		"signupOIDC": (*signupFixture).signupOIDC,
	} {
		t.Run(name+" current", func(t *testing.T) {
			f := newSignupFixture(t, "s")
			f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))

			u, err := signup(f, tosIn("1.0.0"))
			require.NoError(t, err)
			assert.Equal(t, []legal.Acceptance{{AcceptedAt: f.clock, Client: testClient, Ref: tosRef("1.0.0"), UserID: u.ID()}}, f.records(t, u.ID()))
			assert.Empty(t, f.pending(t, u.ID()))
		})
		t.Run(name+" upcoming", func(t *testing.T) {
			f := newSignupFixture(t, "s")
			f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))
			f.publishTos(t, "2.0.0", f.clock.Add(24*time.Hour))

			u, err := signup(f, tosIn("2.0.0"))
			require.NoError(t, err)
			assert.Equal(t, []legal.Ref{tosRef("2.0.0")}, refsOf(f.records(t, u.ID())))
			assert.Empty(t, f.pending(t, u.ID()))
		})
	}
}

func TestUser_SignupRejectsPoliciesBeforeCreatingTheUser(t *testing.T) {
	for name, tc := range map[string]struct {
		in   legal.RefInput
		want error
	}{
		"stale version":     {in: tosIn("1.0.0"), want: legal.ErrNotAcceptable},
		"unknown type":      {in: legal.RefInput{Type: "cookies", Version: "1.0.0"}, want: legal.ErrUnknownType},
		"malformed version": {in: tosIn("1.0"), want: legal.ErrInvalidVersion},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSignupFixture(t, "s")
			f.publishTos(t, "1.0.0", f.clock.Add(-2*time.Hour))
			f.publishTos(t, "1.1.0", f.clock.Add(-time.Hour))

			_, err := f.signup(tc.in)
			assert.ErrorIs(t, err, tc.want)
			_, err = f.signupOIDC(tc.in)
			assert.ErrorIs(t, err, tc.want)
			f.assertNoUser(t)
		})
	}
}

func TestUser_SignupReloadsTheCatalogBeforeRejecting(t *testing.T) {
	f := newSignupFixture(t, "s")
	f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))
	_, err := f.policy.catalog.Get(context.Background())
	require.NoError(t, err)
	f.publishTos(t, "1.1.0", f.clock.Add(-time.Minute))

	u, err := f.signup(tosIn("1.1.0"))
	require.NoError(t, err)
	assert.Equal(t, []legal.Ref{tosRef("1.1.0")}, refsOf(f.records(t, u.ID())))
}

type failingAcceptanceRepo struct {
	legal.AcceptanceRepo
}

func (failingAcceptanceRepo) InsertIfAbsent(context.Context, []legal.Acceptance) error {
	return errors.New("insert failed")
}

func TestUser_SignupSucceedsWhenRecordingPoliciesFails(t *testing.T) {
	f := newSignupFixture(t, "s")
	f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))
	f.repos.PolicyAcceptance = failingAcceptanceRepo{f.repos.PolicyAcceptance}

	u, err := f.signup(tosIn("1.0.0"))
	require.NoError(t, err)
	stored, err := f.repos.User.FindByID(context.Background(), u.ID())
	require.NoError(t, err)
	assert.Equal(t, u.ID(), stored.ID())
	assert.Empty(t, f.records(t, u.ID()))
	assert.Equal(t, []legal.Ref{tosRef("1.0.0")}, f.pending(t, u.ID()))
}

func TestUser_SignupWithoutPolicySecret(t *testing.T) {
	f := newSignupFixture(t, "")
	f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))

	for _, in := range []legal.RefInput{tosIn("1.0.0"), tosIn("1.0")} {
		_, err := f.signup(in)
		assert.ErrorIs(t, err, interfaces.ErrSignupPoliciesNotConfigured)
		_, err = f.signupOIDC(in)
		assert.ErrorIs(t, err, interfaces.ErrSignupPoliciesNotConfigured)
	}
	f.assertNoUser(t)

	u, err := f.signup()
	require.NoError(t, err)
	assert.Empty(t, f.records(t, u.ID()))
}

func TestUser_SignupWithoutPoliciesRecordsNothing(t *testing.T) {
	f := newSignupFixture(t, "s")
	f.publishTos(t, "1.0.0", f.clock.Add(-time.Hour))

	u, err := f.signupOIDC()
	require.NoError(t, err)
	assert.Empty(t, f.records(t, u.ID()))
	assert.Equal(t, []legal.Ref{tosRef("1.0.0")}, f.pending(t, u.ID()))
}
