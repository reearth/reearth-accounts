package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mailprovider"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/maintenance"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestREST_MaintenanceSendsPolicyNotices(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			ctx := context.Background()
			var repos *repo.Container
			if useMongo {
				db := mongorepo.Connect(t)(t)
				require.NoError(t, mongorepo.EnsureIndexes(ctx, db))
				var err error
				repos, err = mongorepo.New(ctx, db, false, false, nil)
				require.NoError(t, err)
			} else {
				repos = memory.New()
			}

			now := time.Now().UTC()
			tos2 := legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}
			for _, e := range []legal.Entry{
				{Ref: legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.0.0")}, EffectiveAt: now.Add(-24 * time.Hour), Required: true},
				{Ref: tos2, EffectiveAt: now.Add(30 * 24 * time.Hour), Required: true,
					Title: legal.LocalizedText{"en": "Terms of Service", "ja": "利用規約"}},
			} {
				_, err := legal.Apply(ctx, repos.PolicyVersion, legal.Manifest{Timestamp: now, Entries: []legal.Entry{e}}, func() time.Time { return now })
				require.NoError(t, err)
			}
			var active []user.ID
			var ja user.ID
			for _, lang := range []string{"ja", "en", ""} {
				m := user.NewMetadata()
				m.LangFrom(lang)
				u := user.New().NewID().Name("u" + lang).Email("u" + lang + "@example.com").Workspace(id.NewWorkspaceID()).Metadata(m).MustBuild()
				require.NoError(t, repos.User.Save(ctx, u))
				active = append(active, u.ID())
				if lang == "ja" {
					ja = u.ID()
				}
			}
			gone := user.New().NewID().Name("gone").Email("gone@example.com").Workspace(id.NewWorkspaceID()).MustBuild()
			gone.Deactivate()
			require.NoError(t, repos.User.Save(ctx, gone))

			heads, err := repos.PolicyVersion.FindHeads(ctx)
			require.NoError(t, err)
			n, err := legal.NewNotice(legal.NewCatalog(heads), "tos", legal.AudienceAll, id.NewAdminUserID(), now)
			require.NoError(t, err)
			_, _, err = repos.PolicyNotice.Create(ctx, n)
			require.NoError(t, err)

			mock := &mailprovider.Mock{}
			exp := StartServerWithGateways(t, maintenanceConfig(maintenanceKey), repos, &gateway.Container{Mail: mock}, true)
			body := exp.POST("/api/maintenance/run").WithHeader(maintenance.KeyHeader, maintenanceKey).
				Expect().Status(http.StatusOK).JSON().Object()
			body.Value("queued").Number().IsEqual(len(active))
			body.Value("sent").Number().IsEqual(len(active))

			for _, u := range active {
				r, err := repos.Outbox.FindByKey(ctx, outbox.PolicyNoticeKey(tos2, u))
				require.NoError(t, err)
				assert.Equal(t, outbox.StateSent, r.State)
			}
			_, err = repos.Outbox.FindByKey(ctx, outbox.PolicyNoticeKey(tos2, gone.ID()))
			assert.Error(t, err, "a deleted user gets no record")

			var japanese int
			for _, e := range mock.Envelopes() {
				if *e.To.UserID() == ja {
					japanese++
					assert.Equal(t, "【Re:Earth】利用規約改定のお知らせ", e.Message.Subject())
				}
			}
			assert.Equal(t, 1, japanese, "the ja user gets one Japanese mail")
			assert.Len(t, mock.Envelopes(), len(active))
		})
	}
}
