package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const webhookSecret = "e2e-policy-webhook-secret"

func webhookPayload(ts time.Time, tosVersion string, tosEffective time.Time) string {
	return fmt.Sprintf(`{
  "event": "policy_version_updated",
  "timestamp": %q,
  "changes": [{"type": "tos"}],
  "versions": {
    "tos": {
      "version": %q,
      "effectiveDate": %q,
      "required": true,
      "title": {"en": "Terms of Service", "ja": "利用規約"},
      "url": {"en": "https://example.com/tos"}
    },
    "privacy": {
      "version": "1.0.0",
      "effectiveDate": "2020-01-01T00:00:00Z",
      "required": true,
      "title": {"en": "Privacy Policy"}
    }
  }
}`, ts.UTC().Format(time.RFC3339), tosVersion, tosEffective.UTC().Format(time.RFC3339))
}

func storedVersions(t *testing.T, r *repo.Container) []string {
	t.Helper()
	recs, err := r.PolicyVersion.FindVersions(context.Background())
	require.NoError(t, err)
	out := []string{}
	for _, rec := range recs {
		out = append(out, string(rec.Type)+"@"+rec.Version.String())
	}
	return out
}

func TestREST_PolicyWebhook(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testPolicyWebhook(t, useMongo)
		})
	}
}

func testPolicyWebhook(t *testing.T, useMongo bool) {
	secret := legal.NewSecret(webhookSecret)
	now := time.Now().Truncate(time.Second)
	upcoming := webhookPayload(now.Add(-time.Minute), "2.0.0", now.Add(30*24*time.Hour))

	t.Run("no secret returns 404 and stores nothing", func(t *testing.T) {
		exp, r := StartServer(t, &app.Config{}, useMongo, nil)
		exp.POST("/api/policies/webhook").
			WithHeader(legal.SignatureHeader, secret.Sign([]byte(upcoming))).
			WithText(upcoming).
			Expect().Status(http.StatusNotFound)
		assert.Empty(t, storedVersions(t, r))
	})

	exp, r := StartServer(t, &app.Config{PolicyWebhookSecret: webhookSecret}, useMongo, nil)
	post := func(body, sig string) int {
		req := exp.POST("/api/policies/webhook").WithText(body)
		if sig != "" {
			req = req.WithHeader(legal.SignatureHeader, sig)
		}
		return req.Expect().Raw().StatusCode
	}

	t.Run("signature checks", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, post(upcoming, ""), "missing")
		assert.Equal(t, http.StatusUnauthorized, post(upcoming, "none"), "none")
		assert.Equal(t, http.StatusUnauthorized, post(upcoming, legal.NewSecret("other").Sign([]byte(upcoming))), "mismatch")
		assert.Equal(t, http.StatusUnauthorized, post(upcoming+" ", secret.Sign([]byte(upcoming))), "other bytes")
		assert.Empty(t, storedVersions(t, r))
	})

	t.Run("oversized body returns 413", func(t *testing.T) {
		big := upcoming[:len(upcoming)-1] + `,"pad":"` + strings.Repeat("x", legal.MaxWebhookBody) + `"}`
		assert.Equal(t, http.StatusRequestEntityTooLarge, post(big, secret.Sign([]byte(big))))
		assert.Empty(t, storedVersions(t, r))
	})

	t.Run("payload checks return 400", func(t *testing.T) {
		future := webhookPayload(now.Add(10*time.Minute), "2.0.0", now.Add(30*24*time.Hour))
		assert.Equal(t, http.StatusBadRequest, post(future, secret.Sign([]byte(future))), "future timestamp")

		missing := strings.Replace(upcoming, `"version": "2.0.0",`, "", 1)
		require.NotEqual(t, upcoming, missing)
		assert.Equal(t, http.StatusBadRequest, post(missing, secret.Sign([]byte(missing))), "missing tos version")
		assert.Empty(t, storedVersions(t, r), "a rejected payload saves no type, the valid privacy entry included")
	})

	t.Run("valid payload is stored and a replay changes nothing", func(t *testing.T) {
		obj := exp.POST("/api/policies/webhook").
			WithHeader(legal.SignatureHeader, secret.Sign([]byte(upcoming))).
			WithText(upcoming).
			Expect().Status(http.StatusOK).JSON().Object()
		obj.Value("applied").Array().ContainsOnly("privacy", "tos")
		obj.Value("ignored").Array().IsEmpty()
		assert.ElementsMatch(t, []string{"privacy@1.0.0", "tos@2.0.0"}, storedVersions(t, r))

		heads, err := r.PolicyVersion.FindHeads(context.Background())
		require.NoError(t, err)

		obj = exp.POST("/api/policies/webhook").
			WithHeader(legal.SignatureHeader, secret.Sign([]byte(upcoming))).
			WithText(upcoming).
			Expect().Status(http.StatusOK).JSON().Object()
		obj.Value("applied").Array().IsEmpty()
		obj.Value("ignored").Array().ContainsOnly("privacy", "tos")

		again, err := r.PolicyVersion.FindHeads(context.Background())
		require.NoError(t, err)
		assert.ElementsMatch(t, heads, again)
	})

	t.Run("a newer revert withdraws the upcoming version", func(t *testing.T) {
		revert := webhookPayload(now, "1.0.0", now.Add(-24*time.Hour))
		assert.Equal(t, http.StatusOK, post(revert, secret.Sign([]byte(revert))))
		heads, err := r.PolicyVersion.FindHeads(context.Background())
		require.NoError(t, err)
		recs, err := r.PolicyVersion.FindVersions(context.Background())
		require.NoError(t, err)
		cat := legal.NewCatalog(heads, recs)
		assert.True(t, cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}))
		assert.Equal(t, "1.0.0", cat.Current("tos", time.Now()).Version.String())
		assert.Nil(t, cat.Upcoming("tos", time.Now()))
	})
}
