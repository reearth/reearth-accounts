package policyuc

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotices(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	versions := memory.NewPolicyVersion()
	publish := func(typ, v string, eff time.Time, required bool) {
		e := legal.Entry{Ref: legal.Ref{Type: legal.Type(typ), Version: legal.MustParseVersion(v)}, EffectiveAt: eff, Required: required}
		_, err := legal.Apply(ctx, versions, legal.Manifest{Timestamp: now, Entries: []legal.Entry{e}}, func() time.Time { return now })
		require.NoError(t, err)
	}
	publish("tos", "1.0.0", now.Add(-time.Hour), true)
	publish("tos", "2.0.0", now.Add(time.Hour), true)
	publish("privacy", "1.0.0", now.Add(-time.Hour), true)
	publish("privacy", "1.0.1", now.Add(time.Hour), true)
	publish("cookies", "2.0.0", now.Add(time.Hour), false)
	notices, ob := memory.NewPolicyNotice(), memory.NewOutbox()
	create, list := NewCreateNoticeUseCase(versions, notices), NewListNoticesUseCase(versions, notices, ob)
	create.now, list.now = func() time.Time { return now }, func() time.Time { return now }
	admin := id.NewAdminUserID()

	_, _, err := create.Execute(ctx, CreateNoticeInput{Audience: legal.AudienceAll, RequestedBy: admin, Type: "privacy"})
	assert.ErrorIs(t, err, legal.ErrPatchNotice)
	_, _, err = create.Execute(ctx, CreateNoticeInput{Audience: legal.AudienceAll, RequestedBy: admin, Type: "cookies"})
	assert.ErrorIs(t, err, legal.ErrNotRequired)

	n, created, err := create.Execute(ctx, CreateNoticeInput{Audience: legal.AudienceOwners, RequestedBy: admin, Type: "tos"})
	require.NoError(t, err)
	assert.True(t, created)
	rec, err := outbox.New(outbox.PolicyNoticeKey(n.Ref, id.NewUserID()), mail.KindPolicyNotice,
		outbox.PolicyNoticeSubject{Bump: n.Bump, Notice: n.ID, Ref: n.Ref}.Subject(), outbox.ToUser(id.NewUserID()), now)
	require.NoError(t, err)
	require.NoError(t, ob.Enqueue(ctx, []outbox.Record{rec}))

	got, err := list.Execute(ctx, "tos")
	require.NoError(t, err)
	want := outbox.EmptyCounts()
	want[outbox.StatePending] = 1
	assert.Equal(t, []NoticeStatus{{Counts: want, Notice: n}}, got)

	got, err = list.Execute(ctx, "privacy")
	require.NoError(t, err)
	assert.Empty(t, got, "a type without notices lists none")
	list.now = func() time.Time { return now.Add(2 * time.Hour) }
	got, err = list.Execute(ctx, "tos")
	require.NoError(t, err)
	assert.Empty(t, got, "once the version takes effect it is not upcoming")
}
