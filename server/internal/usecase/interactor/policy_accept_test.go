package interactor

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type acceptFixture struct {
	clock  time.Time
	op     *workspace.Operator
	policy *Policy
	repos  *repo.Container
	uid    id.UserID
}

func newAcceptFixture(t *testing.T, secret string) *acceptFixture {
	t.Helper()
	r := memory.New()
	u := user.New().NewID().Name("u").Email("u@example.com").Workspace(id.NewWorkspaceID()).MustBuild()
	require.NoError(t, r.User.Save(context.Background(), u))
	uid := u.ID()
	f := &acceptFixture{
		clock: time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC),
		op:    &workspace.Operator{User: &uid},
		repos: r,
		uid:   uid,
	}
	cat := NewPolicyCatalog(r.PolicyVersion, PolicyCatalogTTL)
	cat.now = func() time.Time { return f.clock }
	f.policy = NewPolicy(r, legal.NewSecret(secret), cat)
	f.policy.now = func() time.Time { return f.clock }
	return f
}

func (f *acceptFixture) records(t *testing.T) []legal.Acceptance {
	t.Helper()
	as, err := f.repos.PolicyAcceptance.FindByUser(context.Background(), f.uid)
	require.NoError(t, err)
	return as
}

func tosRef(version string) legal.Ref {
	return legal.Ref{Type: "tos", Version: legal.MustParseVersion(version)}
}

var testClient = legal.Client{IP: netip.MustParseAddr("192.0.2.1"), Origin: "https://app.example.com", UserAgent: "test"}

func TestPolicy_AcceptRecordsOnceWithTheClient(t *testing.T) {
	ctx := context.Background()
	f := newAcceptFixture(t, "s")
	applyTos(t, f.repos.PolicyVersion, f.clock, "1.0.0")

	u, err := f.policy.Accept(ctx, []legal.Ref{tosRef("1.0.0")}, testClient, f.op)
	require.NoError(t, err)
	assert.Equal(t, f.uid, u.ID())
	assert.Equal(t, []legal.Acceptance{{AcceptedAt: f.clock, Client: testClient, Ref: tosRef("1.0.0"), UserID: f.uid}}, f.records(t))

	ov, err := f.policy.Overview(ctx, f.op)
	require.NoError(t, err)
	require.Len(t, ov.Acceptances, 1)
	assert.Equal(t, tosRef("1.0.0"), ov.Acceptances[0].Ref)

	f.clock = f.clock.Add(time.Minute)
	_, err = f.policy.Accept(ctx, []legal.Ref{tosRef("1.0.0")}, legal.Client{UserAgent: "again"}, f.op)
	require.NoError(t, err)
	assert.Len(t, f.records(t), 1)
	assert.Equal(t, testClient, f.records(t)[0].Client, "a repeat writes no record")
}

func TestPolicy_AcceptReloadsTheCatalogBeforeRejecting(t *testing.T) {
	ctx := context.Background()
	f := newAcceptFixture(t, "s")
	applyTos(t, f.repos.PolicyVersion, f.clock, "1.0.0")
	_, err := f.policy.Overview(ctx, f.op)
	require.NoError(t, err)

	// Another instance received 1.1.0; this instance's cache still has 1.0.0.
	applyTos(t, f.repos.PolicyVersion, f.clock.Add(time.Second), "1.1.0")
	cached, err := f.policy.catalog.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", currentTos(t, cached, f.clock))
	_, err = f.policy.Accept(ctx, []legal.Ref{tosRef("1.1.0")}, testClient, f.op)
	require.NoError(t, err)
	assert.Equal(t, tosRef("1.1.0"), f.records(t)[0].Ref)
}

func TestPolicy_AcceptRejects(t *testing.T) {
	ctx := context.Background()
	f := newAcceptFixture(t, "s")
	applyTos(t, f.repos.PolicyVersion, f.clock, "1.0.0")
	applyTos(t, f.repos.PolicyVersion, f.clock.Add(time.Second), "1.1.0")

	for name, tc := range map[string]struct {
		refs []legal.Ref
		want error
	}{
		"unknown type":    {refs: []legal.Ref{{Type: "cookies", Version: legal.MustParseVersion("1.0.0")}}, want: legal.ErrUnknownType},
		"stale version":   {refs: []legal.Ref{tosRef("1.0.0")}, want: legal.ErrNotAcceptable},
		"never published": {refs: []legal.Ref{tosRef("0.9.0")}, want: legal.ErrNotAcceptable},
		"one bad ref":     {refs: []legal.Ref{tosRef("1.1.0"), tosRef("0.9.0")}, want: legal.ErrNotAcceptable},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.policy.Accept(ctx, tc.refs, testClient, f.op)
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, f.records(t))
		})
	}
}

func TestPolicy_AcceptRefusesAServiceCaller(t *testing.T) {
	f := newAcceptFixture(t, "s")
	applyTos(t, f.repos.PolicyVersion, f.clock, "1.0.0")
	svc := &workspace.Operator{Integration: id.NewIntegrationID().Ref()}

	_, err := f.policy.Accept(context.Background(), []legal.Ref{tosRef("1.0.0")}, testClient, svc)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
	_, err = f.policy.Overview(context.Background(), svc)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
}

func TestPolicy_WithoutSecret(t *testing.T) {
	ctx := context.Background()
	f := newAcceptFixture(t, "")
	applyTos(t, f.repos.PolicyVersion, f.clock, "1.0.0")

	_, err := f.policy.Accept(ctx, []legal.Ref{tosRef("1.0.0")}, testClient, f.op)
	assert.ErrorIs(t, err, legal.ErrNotConfigured)
	assert.Empty(t, f.records(t))

	ov, err := f.policy.Overview(ctx, f.op)
	require.NoError(t, err)
	assert.Equal(t, legal.EmptyOverview(), ov)
}
