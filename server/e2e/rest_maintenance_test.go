package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/maintenance"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maintenanceKey = "e2e-maintenance-key"

func maintenanceConfig(key string) *app.Config {
	return &app.Config{MaintenanceKey: key, MailerMaxAttempts: 5, MailerOutboxTTL: 720 * time.Hour, MailerRate: 50}
}

func TestREST_MaintenanceRun(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			t.Run("no key returns 404", func(t *testing.T) {
				exp, _ := StartServer(t, maintenanceConfig(""), useMongo, nil)
				exp.POST("/api/maintenance/run").WithHeader(maintenance.KeyHeader, "anything").
					Expect().Status(http.StatusNotFound)
			})

			exp, r := StartServer(t, maintenanceConfig(maintenanceKey), useMongo, nil)

			t.Run("wrong or missing key returns 401", func(t *testing.T) {
				exp.POST("/api/maintenance/run").WithHeader(maintenance.KeyHeader, "wrong").
					Expect().Status(http.StatusUnauthorized)
				exp.POST("/api/maintenance/run").Expect().Status(http.StatusUnauthorized)
			})

			t.Run("right key runs a pass", func(t *testing.T) {
				ctx := context.Background()
				rec, err := outbox.New("e2e:1", "no_such_kind", nil, outbox.ToUser(id.NewUserID()), time.Now().Add(-time.Second))
				require.NoError(t, err)
				require.NoError(t, r.Outbox.Enqueue(ctx, []outbox.Record{rec}))

				body := exp.POST("/api/maintenance/run").WithHeader(maintenance.KeyHeader, maintenanceKey).
					Expect().Status(http.StatusOK).JSON().Object()
				body.Value("ran").Boolean().IsTrue()
				body.Value("failed").Number().IsEqual(1)
				body.Value("sent").Number().IsEqual(0)
				body.Value("retained").Object().ContainsKey("orphan_policy_acceptances")

				got, err := r.Outbox.FindByKey(ctx, "e2e:1")
				require.NoError(t, err)
				assert.Equal(t, outbox.StateFailed, got.State)
				assert.Equal(t, outbox.ReasonUnknownKind, got.Reason)
			})
		})
	}
}
