package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyOrigin    = "https://app.example.com"
	policyUserAgent = "policy-e2e"
	forgedIP        = "198.51.100.9"

	tosCurrent       = "1.0.0"
	tosUpcoming      = "2.0.0"
	tosUnpublished   = "2.5.0"
	privacyCurrent   = "1.0.0"
	inEffectSince    = -24 * time.Hour
	upcomingIn       = 30 * 24 * time.Hour
	malformedVersion = "1.0"

	meAllPolicies = `{ me {
		pendingPolicies { type version title url required effectiveDate }
		upcomingPolicies { type version title url }
		policyAcceptances { type version acceptedAt }
	} }`
	acceptMutation = `mutation($input: [PolicyAcceptanceInput!]!) {
		acceptPolicies(input: $input) { me { pendingPolicies { type } policyAcceptances { type version } } }
	}`
)

var (
	policyUser = id.NewUserID()
	policyWS   = id.NewWorkspaceID()
)

func seedPolicyUser(ctx context.Context, r *repo.Container) error {
	md := user.NewMetadata()
	md.LangFrom("ja")
	u := user.New().ID(policyUser).Name("policy").Email("policy@example.com").Metadata(md).
		Auths([]user.Auth{*user.ReearthSub(policyUser.String())}).Workspace(policyWS).MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}
	return r.Workspace.Save(ctx, workspace.New().ID(policyWS).Name("policy").Personal(true).
		Members(map[id.UserID]workspace.Member{policyUser: {Role: role.RoleOwner, InvitedBy: policyUser}}).MustBuild())
}

func deliverPolicies(t *testing.T, e *httpexpect.Expect, body string) {
	t.Helper()
	e.POST("/api/policies/webhook").
		WithHeader(legal.SignatureHeader, legal.NewSecret(webhookSecret).Sign([]byte(body))).
		WithText(body).
		Expect().Status(http.StatusOK)
}

func policyGQL(e *httpexpect.Expect, query string, vars map[string]any) *httpexpect.Object {
	return e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", policyUser.String()).
		WithHeader("Origin", policyOrigin).
		WithHeader("User-Agent", policyUserAgent).
		WithHeader("X-Forwarded-For", forgedIP).
		WithJSON(GraphQLRequest{Query: query, Variables: vars}).
		Expect().Status(http.StatusOK).JSON().Object()
}

func demoUserREST(r *httpexpect.Request) *httpexpect.Request {
	return r.WithHeader("Origin", policyOrigin).
		WithHeader("User-Agent", policyUserAgent).
		WithHeader("X-Forwarded-For", forgedIP)
}

func acceptInput(refs ...string) map[string]any {
	in := []map[string]string{}
	for i := 0; i < len(refs); i += 2 {
		in = append(in, map[string]string{"type": refs[i], "version": refs[i+1]})
	}
	return map[string]any{"input": in}
}

func storedAcceptances(t *testing.T, r *repo.Container, u id.UserID) []legal.Acceptance {
	t.Helper()
	as, err := r.PolicyAcceptance.FindByUser(context.Background(), u)
	require.NoError(t, err)
	return as
}

func currentTosPayload(now time.Time, delivered time.Duration) string {
	return webhookPayload(now.Add(delivered), tosCurrent, now.Add(inEffectSince))
}

func upcomingTosPayload(now time.Time) string {
	return webhookPayload(now.Add(-time.Minute), tosUpcoming, now.Add(upcomingIn))
}

func TestPolicyAcceptance_GraphQL(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testPolicyAcceptanceGraphQL(t, useMongo)
		})
	}
}

func testPolicyAcceptanceGraphQL(t *testing.T, useMongo bool) {
	now := time.Now().Truncate(time.Second)
	e, r := StartServer(t, &app.Config{PolicyWebhookSecret: webhookSecret}, useMongo, seedPolicyUser)

	deliverPolicies(t, e, currentTosPayload(now, -2*time.Minute))

	me := policyGQL(e, meAllPolicies, nil).Value("data").Object().Value("me").Object()
	pending := me.Value("pendingPolicies").Array()
	pending.Length().IsEqual(2)
	pending.Value(0).Object().HasValue("type", "privacy").HasValue("title", "Privacy Policy").Value("url").IsNull()
	pending.Value(1).Object().HasValue("type", "tos").HasValue("version", tosCurrent).
		HasValue("title", "利用規約").HasValue("url", "https://example.com/tos").HasValue("required", true)
	me.Value("upcomingPolicies").Array().IsEmpty()
	me.Value("policyAcceptances").Array().IsEmpty()

	deliverPolicies(t, e, upcomingTosPayload(now))
	upcoming := policyGQL(e, meAllPolicies, nil).Value("data").Object().Value("me").Object().Value("upcomingPolicies").Array()
	upcoming.Length().IsEqual(1)
	upcoming.Value(0).Object().HasValue("type", "tos").HasValue("version", tosUpcoming)

	t.Run("accept records once with the client", func(t *testing.T) {
		payload := policyGQL(e, acceptMutation, acceptInput("tos", tosCurrent, "privacy", privacyCurrent)).
			Value("data").Object().Value("acceptPolicies").Object().Value("me").Object()
		payload.Value("pendingPolicies").Array().IsEmpty()
		payload.Value("policyAcceptances").Array().IsEqual([]map[string]string{
			{"type": "privacy", "version": privacyCurrent},
			{"type": "tos", "version": tosCurrent},
		})

		as := storedAcceptances(t, r, policyUser)
		require.Len(t, as, 2)
		for _, a := range as {
			assert.Equal(t, policyOrigin, a.Client.Origin)
			assert.Equal(t, policyUserAgent, a.Client.UserAgent)
			assert.True(t, a.Client.IP.IsLoopback(), "the connection address, not X-Forwarded-For from an untrusted peer: %s", a.Client.IP)
		}

		policyGQL(e, acceptMutation, acceptInput("tos", tosCurrent)).NotContainsKey("errors")
		assert.Len(t, storedAcceptances(t, r, policyUser), 2, "a repeat writes no record")
	})

	t.Run("upcoming acceptance is recorded and counts for current", func(t *testing.T) {
		policyGQL(e, acceptMutation, acceptInput("tos", tosUpcoming)).NotContainsKey("errors")
		assert.Len(t, storedAcceptances(t, r, policyUser), 3)
		me := policyGQL(e, meAllPolicies, nil).Value("data").Object().Value("me").Object()
		me.Value("pendingPolicies").Array().IsEmpty()
		me.Value("policyAcceptances").Array().Value(1).Object().HasValue("version", tosUpcoming)

		policyGQL(e, acceptMutation, acceptInput("tos", tosCurrent)).NotContainsKey("errors")
		assert.Len(t, storedAcceptances(t, r, policyUser), 3, "a lower version writes no record")
	})

	t.Run("a version neither current nor upcoming is refused", func(t *testing.T) {
		for _, in := range []map[string]any{
			acceptInput("tos", tosUnpublished),
			acceptInput("cookies", tosCurrent),
			acceptInput("privacy", privacyCurrent, "tos", tosUnpublished),
			acceptInput("TOS!", tosCurrent),
		} {
			policyGQL(e, acceptMutation, in).Value("errors").Array().NotEmpty()
		}
		policyGQL(e, acceptMutation, acceptInput("tos", malformedVersion)).Value("errors").Array().
			Value(0).Object().Value("message").String().Contains("MAJOR.MINOR.PATCH")
		assert.Len(t, storedAcceptances(t, r, policyUser), 3)
	})

	t.Run("deleteMe removes the records", func(t *testing.T) {
		policyGQL(e, fmt.Sprintf(`mutation { deleteMe(input: {userId: "%s"}) { userId } }`, policyUser), nil).
			NotContainsKey("errors")
		assert.Empty(t, storedAcceptances(t, r, policyUser))
	})
}

func TestPolicyAcceptance_REST(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		t.Run(fmt.Sprintf("mongo=%v", useMongo), func(t *testing.T) {
			testPolicyAcceptanceREST(t, useMongo)
		})
	}
}

func testPolicyAcceptanceREST(t *testing.T, useMongo bool) {
	now := time.Now().Truncate(time.Second)
	e, r := StartServer(t, &app.Config{Mock_Auth: true, PolicyWebhookSecret: webhookSecret}, useMongo, seedDemoUser)
	deliverPolicies(t, e, upcomingTosPayload(now))

	got := demoUserREST(e.GET("/api/users/me/policies")).Expect().Status(http.StatusOK).JSON().Object()
	got.Value("acceptances").Array().IsEmpty()
	got.Value("pending").Array().Length().IsEqual(1)
	got.Value("pending").Array().Value(0).Object().HasValue("type", "privacy")
	got.Value("upcoming").Array().Value(0).Object().
		HasValue("type", "tos").HasValue("version", tosUpcoming).HasValue("title", "Terms of Service")

	accepted := demoUserREST(e.POST("/api/users/me/policies/accept")).
		WithJSON(map[string]any{"policies": acceptInput("tos", tosUpcoming, "privacy", privacyCurrent)["input"]}).
		Expect().Status(http.StatusOK).JSON().Object()
	accepted.Value("pending").Array().IsEmpty()
	acc := accepted.Value("acceptances").Array()
	acc.Length().IsEqual(2)
	acc.Value(1).Object().Keys().ContainsOnly("acceptedAt", "type", "version")
	acc.Value(1).Object().HasValue("type", "tos").HasValue("version", tosUpcoming)

	as := storedAcceptances(t, r, restDemoUID)
	require.Len(t, as, 2)
	assert.True(t, as[0].Client.IP.IsLoopback())
	assert.Equal(t, policyUserAgent, as[0].Client.UserAgent)

	for _, body := range []any{
		map[string]any{"policies": acceptInput("tos", tosUnpublished)["input"]},
		map[string]any{"policies": acceptInput("cookies", tosCurrent)["input"]},
		map[string]any{"policies": acceptInput("tos", malformedVersion)["input"]},
		map[string]any{},
	} {
		demoUserREST(e.POST("/api/users/me/policies/accept")).WithJSON(body).
			Expect().Status(http.StatusBadRequest)
	}
	demoUserREST(e.POST("/api/users/me/policies/accept")).
		WithJSON(map[string]any{"policies": acceptInput("tos", malformedVersion)["input"]}).
		Expect().Status(http.StatusBadRequest).JSON().Object().Value("message").String().Contains("MAJOR.MINOR.PATCH")
	assert.Len(t, storedAcceptances(t, r, restDemoUID), 2)

	demoUserREST(e.DELETE("/api/users/me")).Expect().Status(http.StatusNoContent)
	assert.Empty(t, storedAcceptances(t, r, restDemoUID))
}

func TestPolicyAcceptance_WithdrawingTheUpcomingKeepsTheCurrent(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	e, _ := StartServer(t, &app.Config{Mock_Auth: true, PolicyWebhookSecret: webhookSecret}, false, seedDemoUser)
	deliverPolicies(t, e, currentTosPayload(now, -2*time.Minute))
	deliverPolicies(t, e, upcomingTosPayload(now))

	demoUserREST(e.POST("/api/users/me/policies/accept")).
		WithJSON(map[string]any{"policies": acceptInput("tos", tosCurrent, "tos", tosUpcoming, "privacy", privacyCurrent)["input"]}).
		Expect().Status(http.StatusOK)
	deliverPolicies(t, e, currentTosPayload(now, 0))

	got := demoUserREST(e.GET("/api/users/me/policies")).Expect().Status(http.StatusOK).JSON().Object()
	got.Value("upcoming").Array().IsEmpty()
	got.Value("pending").Array().IsEmpty()
	got.Value("acceptances").Array().Value(1).Object().HasValue("type", "tos").HasValue("version", tosCurrent)
}

func TestPolicyAcceptance_WithoutSecret(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, false, seedPolicyUser)

	me := policyGQL(e, meAllPolicies, nil).Value("data").Object().Value("me").Object()
	me.Value("pendingPolicies").Array().IsEmpty()
	me.Value("upcomingPolicies").Array().IsEmpty()
	me.Value("policyAcceptances").Array().IsEmpty()

	for _, v := range []string{tosCurrent, malformedVersion} {
		errs := policyGQL(e, acceptMutation, acceptInput("tos", v)).Value("errors").Array()
		errs.Value(0).Object().Value("message").String().Contains("policies not configured")
	}

	rest, restRepos := StartServer(t, &app.Config{Mock_Auth: true}, false, seedDemoUser)
	demoUserREST(rest.GET("/api/users/me/policies")).Expect().Status(http.StatusOK).JSON().Object().
		IsEqual(map[string]any{"acceptances": []any{}, "pending": []any{}, "upcoming": []any{}})
	for _, v := range []string{tosCurrent, malformedVersion} {
		demoUserREST(rest.POST("/api/users/me/policies/accept")).
			WithJSON(map[string]any{"policies": acceptInput("tos", v)["input"]}).
			Expect().Status(http.StatusNotFound)
	}
	assert.Empty(t, storedAcceptances(t, r, policyUser))
	assert.Empty(t, storedAcceptances(t, restRepos, restDemoUID))
}

func TestPolicyAcceptance_ServiceCallerRefused(t *testing.T) {
	idp := newSvcIdP(t)
	jwks := idp.srv.URL + "/jwks.json"
	cfg := &app.Config{
		Auth:                app.AuthConfigs{{ISS: idp.issuer(), AUD: []string{svcAudience}, JWKSURI: &jwks}},
		PolicyWebhookSecret: webhookSecret,
	}
	callers, err := json.Marshal([]map[string]string{{"issuer": idp.issuer(), "client_id": "svc-flow", "integration_id": svcIntegration.String()}})
	require.NoError(t, err)
	require.NoError(t, cfg.ServiceCallers.Decode(string(callers)))
	e, r := StartServerNoDebug(t, cfg, false, seedServiceCallers)
	deliverPolicies(t, e, upcomingTosPayload(time.Now().Truncate(time.Second)))
	auth := "Bearer " + idp.clientToken(t, "svc-flow")

	serviceGQL(e, auth, acceptMutation, acceptInput("tos", tosCurrent)).Value("errors").Array().NotEmpty()
	e.POST("/api/users/me/policies/accept").WithHeader("Authorization", auth).
		WithJSON(map[string]any{"policies": acceptInput("tos", tosCurrent)["input"]}).
		Expect().Status(http.StatusUnauthorized)
	e.GET("/api/users/me/policies").WithHeader("Authorization", auth).
		Expect().Status(http.StatusUnauthorized)

	as, err := r.PolicyAcceptance.FindByUser(context.Background(), svcOwnerUID)
	require.NoError(t, err)
	assert.Empty(t, as)
}
