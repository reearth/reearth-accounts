package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	auSubject   = id.NewUserID()
	auSubjectWS = id.NewWorkspaceID()
	auShared    = id.NewWorkspaceID()
	auDeleted   = id.NewUserID()
)

const (
	auSubjectEmail = "act-as-subject@e2e.com"
	auSubjectSub   = "auth0|act-as-subject"
	auBilling      = "act-as-billing-sentinel@e2e.com"
)

// The subject owns a personal workspace and shares auShared with the privacy
// target; privTargetWS, privTargetTeamWS and privStrangerWS are not its own.
func seedActAsUser(ctx context.Context, r *repo.Container) error {
	if err := seedPrivacy(ctx, r); err != nil {
		return err
	}
	subject := user.New().ID(auSubject).Name("act-as-subject").Alias("act-as-subject").Email(auSubjectEmail).
		Auths([]user.Auth{{Provider: "auth0", Sub: auSubjectSub}}).Workspace(auSubjectWS).MustBuild()
	deleted := user.New().ID(auDeleted).Name("act-as-deleted").Email("act-as-deleted@e2e.com").Workspace(id.NewWorkspaceID()).MustBuild()
	deleted.Deactivate()
	for _, u := range []*user.User{subject, deleted} {
		if err := r.User.Save(ctx, u); err != nil {
			return err
		}
	}
	md := workspace.NewMetadata()
	md.SetBillingEmail(auBilling)
	for _, w := range []*workspace.Workspace{
		workspace.New().ID(auSubjectWS).Name("subject personal").Alias("act-as-subject-ws").Personal(true).
			Members(map[workspace.UserID]workspace.Member{auSubject: {Role: role.RoleOwner, InvitedBy: auSubject}}).Metadata(md).MustBuild(),
		workspace.New().ID(auShared).Name("shared").Alias("act-as-shared").Metadata(md).
			Members(map[workspace.UserID]workspace.Member{
				auSubject:  {Role: role.RoleWriter, InvitedBy: privTarget},
				privTarget: {Role: role.RoleOwner, InvitedBy: privTarget},
			}).MustBuild(),
	} {
		if err := r.Workspace.Save(ctx, w); err != nil {
			return err
		}
	}
	return nil
}

type actAsUserServer struct {
	e       *httpexpect.Expect
	url     string
	idp     *svcIdP
	users   map[string]string // the user-only client acting for the subject
	both    map[string]string // the client with both kinds acting for the subject
	own     map[string]string // the subject's own token
	intOnly string            // a token of the client with only "integration"
}

func startActAsUserServer(t *testing.T, useMongo bool) actAsUserServer {
	t.Helper()
	idp := newSvcIdP(t)
	jwks := idp.srv.URL + "/jwks.json"
	cfg := &app.Config{Auth: app.AuthConfigs{{ISS: idp.issuer(), AUD: []string{svcAudience}, JWKSURI: &jwks}}}
	callers, err := json.Marshal([]map[string]any{
		{"issuer": idp.issuer(), "client_id": "svc-users", "name": "users-app", "act_as": []string{"user"}},
		{"issuer": idp.issuer(), "client_id": "svc-flow", "name": "flow-app", "act_as": []string{"integration", "user"}},
		{"issuer": idp.issuer(), "client_id": "svc-int", "name": "int-app", "act_as": []string{"integration"}},
	})
	require.NoError(t, err)
	require.NoError(t, cfg.ServiceCallers.Decode(string(callers)))
	url := startServerURL(t, cfg, seededRepos(t, useMongo, seedActAsUser), nil, false)
	asSubject := "user:" + auSubject.String()
	return actAsUserServer{
		e:       httpexpect.Default(t, url),
		url:     url,
		idp:     idp,
		users:   actingFor(idp.clientToken(t, "svc-users"), asSubject),
		both:    actingFor(idp.clientToken(t, "svc-flow"), asSubject),
		own:     actingFor(idp.token(t, jwt.MapClaims{"sub": auSubjectSub})),
		intOnly: idp.clientToken(t, "svc-int"),
	}
}

func (s actAsUserServer) at(t *testing.T) actAsUserServer {
	s.e = httpexpect.Default(t, s.url)
	return s
}

func (s actAsUserServer) callers() map[string]map[string]string {
	return map[string]map[string]string{"user-only client": s.users, "client with both kinds": s.both}
}

func (s actAsUserServer) gql(h map[string]string, q string) (*httpexpect.Object, string) {
	res := s.e.POST("/api/graphql").WithHeaders(h).WithJSON(GraphQLRequest{Query: q}).Expect().Status(http.StatusOK)
	raw := res.Body().Raw()
	return res.JSON().Object(), raw
}

func memberRoles(t *testing.T, raw string) map[string]map[string]string {
	t.Helper()
	var ws []struct {
		ID      string `json:"id"`
		Members []struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		} `json:"members"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &ws), raw)
	out := map[string]map[string]string{}
	for _, w := range ws {
		out[w.ID] = map[string]string{}
		for _, m := range w.Members {
			out[w.ID][m.UserID] = m.Role
		}
	}
	return out
}

func TestActAsUser_ReadList(t *testing.T) {
	s := startActAsUserServer(t, false)
	wantRoles := map[string]map[string]string{
		auSubjectWS.String(): {auSubject.String(): "owner"},
		auShared.String():    {auSubject.String(): "writer", privTarget.String(): "owner"},
	}
	for name, h := range s.callers() {
		t.Run(name+" reads the subject's profile on REST", func(t *testing.T) {
			s := s.at(t)
			me := s.e.GET("/api/users/me").WithHeaders(h).Expect().Status(http.StatusOK).JSON().Object()
			me.HasValue("id", auSubject.String()).HasValue("email", auSubjectEmail)
			ws := s.e.GET("/api/users/me").WithHeaders(h).WithQuery("expand", "my_workspace").
				Expect().Status(http.StatusOK).JSON().Object().Value("my_workspace").Object()
			ws.HasValue("id", auSubjectWS.String())
			ws.Value("metadata").Object().HasValue("billing_email", "")
		})

		t.Run(name+" reads the subject's profile on GraphQL", func(t *testing.T) {
			s := s.at(t)
			o, _ := s.gql(h, `{ me { id email myWorkspaceId myWorkspace { id metadata { billingEmail } } } }`)
			me := o.Value("data").Object().Value("me").Object()
			me.HasValue("id", auSubject.String()).HasValue("email", auSubjectEmail).HasValue("myWorkspaceId", auSubjectWS.String())
			me.Value("myWorkspace").Object().Value("metadata").Object().HasValue("billingEmail", "")
		})

		t.Run(name+" reads the subject's workspaces and roles on REST", func(t *testing.T) {
			s := s.at(t)
			raw := s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("user_id", auSubject.String()).WithQuery("expand", "members.user").
				Expect().Status(http.StatusOK).Body().Raw()
			assert.Equal(t, wantRoles, memberRoles(t, raw))
			assert.NotContains(t, raw, privTargetEmail)
			assert.NotContains(t, raw, auBilling)
			assert.Contains(t, raw, auSubjectEmail, "the subject's own embedded record keeps its email")

			page := s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("user_id", auSubject.String()).WithQuery("page", "1").WithQuery("page_size", "1").
				Expect().Status(http.StatusOK).JSON().Object()
			page.Value("pagination").Object().HasValue("total", 2)
		})

		t.Run(name+" reads the subject's workspaces and roles on GraphQL", func(t *testing.T) {
			s := s.at(t)
			q := `query { findByUser(userId: "` + auSubject.String() + `") { id metadata { billingEmail } members { ... on WorkspaceUserMember { userId role user { id email } } } }
				findByUserWithPagination(userId: "` + auSubject.String() + `", pagination: {page: 1, size: 10}) { totalCount } }`
			o, raw := s.gql(h, q)
			data := o.Value("data").Object()
			assert.ElementsMatch(t, []string{auSubjectWS.String(), auShared.String()}, idsOf(data.Value("findByUser").Array()))
			data.Value("findByUserWithPagination").Object().HasValue("totalCount", 2)
			assert.NotContains(t, raw, privTargetEmail)
			assert.NotContains(t, raw, auBilling)
			assert.Contains(t, raw, `"role":"owner"`)
			assert.Contains(t, raw, privTarget.String())
		})

		t.Run(name+" cannot name another user's workspaces", func(t *testing.T) {
			s := s.at(t)
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("user_id", privTarget.String()).Expect().Status(http.StatusForbidden)
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("user_id", privTarget.String()).WithQuery("page", "1").Expect().Status(http.StatusForbidden)
			for _, q := range []string{
				`{ findByUser(userId: "` + privTarget.String() + `") { id } }`,
				`{ findByUserWithPagination(userId: "` + privTarget.String() + `", pagination: {page: 1, size: 10}) { totalCount } }`,
			} {
				o, raw := s.gql(h, q)
				o.Value("errors").Array().NotEmpty()
				assert.NotContains(t, raw, privTargetTeamWS.String())
			}
		})

		t.Run(name+" sees nothing of a workspace the subject is not in", func(t *testing.T) {
			s := s.at(t)
			raw := s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("ids", privTargetTeamWS.String()+","+auShared.String()).
				Expect().Status(http.StatusOK).Body().Raw()
			assert.Contains(t, raw, auShared.String())
			assert.NotContains(t, raw, privTargetTeamWS.String())
			assert.NotContains(t, raw, "target team")
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("name", "target team").Expect().Status(http.StatusNotFound)
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("alias", "privacy-target-team").Expect().Status(http.StatusNotFound)
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("expand", "members.user").WithQuery("alias", "privacy-target-team").Expect().Status(http.StatusNotFound)
			s.e.GET("/api/workspaces").WithHeaders(h).WithQuery("expand", "my_workspace").WithQuery("ids", auShared.String()).Expect().Status(http.StatusBadRequest)
		})
	}
}

func TestActAsUser_RefusedSubjects(t *testing.T) {
	for _, useMongo := range []bool{false, true} {
		name := "memory"
		if useMongo {
			name = "mongo"
		}
		t.Run(name, func(t *testing.T) {
			s := startActAsUserServer(t, useMongo)
			refused := map[string]map[string]string{
				"a client without user in act_as": actingFor(s.intOnly, "user:"+auSubject.String()),
				"an unknown user":                 actingFor(s.idp.clientToken(t, "svc-users"), "user:"+id.NewUserID().String()),
				"a deactivated user":              actingFor(s.idp.clientToken(t, "svc-users"), "user:"+auDeleted.String()),
			}
			for why, h := range refused {
				t.Run(why, func(t *testing.T) {
					s := s.at(t)
					s.e.GET("/api/users/me").WithHeaders(h).Expect().Status(http.StatusUnauthorized)
					s.e.POST("/api/graphql").WithHeaders(h).WithJSON(GraphQLRequest{Query: `{ me { id } }`}).Expect().Status(http.StatusUnauthorized)
				})
			}
		})
	}
}

func closedToUserClientsREST() [][2]string {
	ws := "/api/workspaces/" + auShared.String()
	var out [][2]string
	for _, r := range closedToClientsREST() {
		if r != [2]string{http.MethodGet, "/api/users/me"} {
			out = append(out, r)
		}
	}
	return append(out, [][2]string{
		{http.MethodGet, "/api/auth/config"},
		{http.MethodPost, "/api/permissions/check"},
		{http.MethodGet, "/api/users/all"},
		{http.MethodPost, "/api/users/" + privTarget.String() + "/deactivate"},
		{http.MethodPost, "/api/users/" + privTarget.String() + "/restore"},
		{http.MethodPatch, "/api/users/by-sub/x"},
		{http.MethodPut, "/api/users/by-sub/x/platform-roles"},
		{http.MethodPost, "/api/users/sync-sso"},
		{http.MethodPost, "/api/users/find-or-create"},
		{http.MethodPost, "/api/workspaces"},
		{http.MethodGet, "/api/workspaces/all"},
		{http.MethodGet, ws},
		{http.MethodPatch, ws},
		{http.MethodDelete, ws},
		{http.MethodPost, ws + "/deactivate"},
		{http.MethodPost, ws + "/restore"},
		{http.MethodPost, ws + "/members"},
		{http.MethodPatch, ws + "/members/" + privTarget.String()},
		{http.MethodDelete, ws + "/members/" + privTarget.String()},
		{http.MethodDelete, ws + "/members"},
		{http.MethodPost, ws + "/integrations"},
		{http.MethodPatch, ws + "/integrations/" + svcIntegration.String()},
		{http.MethodDelete, ws + "/integrations/" + svcIntegration.String()},
		{http.MethodDelete, ws + "/integrations"},
		{http.MethodPost, ws + "/transfer-ownership"},
		{http.MethodGet, ws + "/scim"},
		{http.MethodPatch, ws + "/scim"},
		{http.MethodPatch, "/api/service/workspaces/" + auShared.String() + "/members/" + privTarget.String()},
	}...)
}

func closedToUserClientsGQL() []string {
	var out []string
	for _, q := range closedToClientsGQL {
		if q != `{ me { id } }` {
			out = append(out, q)
		}
	}
	ws, target := auShared.String(), privTarget.String()
	return append(out,
		`{ authConfig { __typename } }`,
		`{ checkPermission(input: {service: "s", resource: "r", action: "read"}) { allowed } }`,
		`{ node(id: "`+target+`", type: USER) { id } }`,
		`{ nodes(id: ["`+target+`"], type: USER) { id } }`,
		`{ node(id: "`+ws+`", type: WORKSPACE) { id } }`,
		`{ user(id: "`+target+`") { id } }`,
		`{ findUsersByIDs(ids: ["`+target+`"]) { id } }`,
		`{ findUsersByIDsWithPagination(ids: ["`+target+`"], pagination: {page: 1, size: 1}) { totalCount } }`,
		`{ userByNameOrAlias(nameOrAlias: "x") { id } }`,
		`{ findByID(id: "`+ws+`") { id } }`,
		`{ findByIDs(ids: ["`+ws+`"]) { id } }`,
		`{ findByName(name: "shared") { id } }`,
		`{ findByAlias(alias: "act-as-shared") { id } }`,
		`{ publicWorkspace(id: "`+ws+`") { id } }`,
		`mutation { createWorkspace(input: {alias: "x", name: "x"}) { workspace { id } } }`,
		`mutation { updateWorkspace(input: {workspaceId: "`+ws+`", name: "x"}) { workspace { id } } }`,
		`mutation { deleteWorkspace(input: {workspaceId: "`+ws+`"}) { workspaceId } }`,
		`mutation { addUsersToWorkspace(input: {workspaceId: "`+ws+`", users: []}) { workspace { id } } }`,
		`mutation { addIntegrationToWorkspace(input: {workspaceId: "`+ws+`", integrationId: "x", role: reader}) { workspace { id } } }`,
		`mutation { removeUserFromWorkspace(input: {workspaceId: "`+ws+`", userId: "`+target+`"}) { workspace { id } } }`,
		`mutation { updateUserOfWorkspace(input: {workspaceId: "`+ws+`", userId: "`+target+`", role: reader}) { workspace { id } } }`,
		`mutation { transferWorkspaceOwnership(input: {workspaceId: "`+ws+`", newOwnerId: "`+target+`"}) { workspace { id } } }`,
		`mutation { findOrCreate(input: {sub: "x", iss: "x", token: "x"}) { user { id } } }`,
	)
}

func TestActAsUser_EverythingElseIsClosed(t *testing.T) {
	s := startActAsUserServer(t, false)
	for name, h := range s.callers() {
		t.Run("REST "+name, func(t *testing.T) {
			s := s.at(t)
			for _, r := range closedToUserClientsREST() {
				body := s.e.Request(r[0], r[1]).WithHeaders(h).WithJSON(map[string]any{}).
					Expect().Status(http.StatusForbidden).Body().Raw()
				assert.Contains(t, body, clientGateRefusal, "%s %s", r[0], r[1])
			}
		})
		t.Run("GraphQL "+name, func(t *testing.T) {
			s := s.at(t)
			for _, q := range closedToUserClientsGQL() {
				refusedGQL(t, s.e, h, q, nil)
			}
		})
	}

	t.Run("the subject's own token is not stopped by the gate", func(t *testing.T) {
		s := s.at(t)
		for _, r := range closedToUserClientsREST() {
			body := s.e.Request(r[0], r[1]).WithHeaders(s.own).WithJSON(map[string]any{}).Expect().Body().Raw()
			assert.NotContains(t, body, clientGateRefusal, "%s %s", r[0], r[1])
		}
		for _, q := range closedToUserClientsGQL() {
			body := s.e.POST("/api/graphql").WithHeaders(s.own).WithJSON(GraphQLRequest{Query: q}).Expect().Body().Raw()
			assert.NotContains(t, body, clientGateRefusal, q)
		}
	})
}

func TestActAsUser_OwnTokenUnchanged(t *testing.T) {
	s := startActAsUserServer(t, false)
	s.e.GET("/api/users/me").WithHeaders(s.own).Expect().Status(http.StatusOK).JSON().Object().HasValue("email", auSubjectEmail)

	raw := s.e.GET("/api/workspaces").WithHeaders(s.own).WithQuery("ids", privTargetTeamWS.String()+","+auShared.String()).
		Expect().Status(http.StatusOK).Body().Raw()
	assert.Contains(t, raw, privTargetTeamWS.String(), "a user's own lookups stay unscoped")
	assert.Contains(t, raw, auBilling, "a member's own token still sees the billing email")

	o, raw := s.gql(s.own, `{ findByUser(userId: "`+privTarget.String()+`") { id metadata { billingEmail } } }`)
	assert.Equal(t, []string{auShared.String()}, idsOf(o.Value("data").Object().Value("findByUser").Array()))
	assert.Contains(t, raw, auBilling)
	s.e.GET("/api/workspaces").WithHeaders(s.own).WithQuery("user_id", privTarget.String()).Expect().Status(http.StatusOK)
}

// Every selection the schema allows below the three read-list fields, as in
// TestGQLPrivacy_NoTargetPrivateFieldsForOthers, run as a client acting for
// the subject, who shares a workspace with the privacy target.
func TestActAsUser_NestedSelectionsReachNoCoMemberData(t *testing.T) {
	s := startActAsUserServer(t, false)
	g := privacyGen{s: gql.NewExecutableSchema(gql.Config{}).Schema()}
	sub := `"` + auSubject.String() + `"`
	calls := map[string]string{
		"me":                       "me",
		"findByUser":               "findByUser(userId: " + sub + ")",
		"findByUserWithPagination": "findByUserWithPagination(userId: " + sub + ", pagination: {page: 1, size: 50})",
	}
	// privTargetWS is not here: a co-member's User.workspace names it, a PUBLIC
	// field anyone can read, and nothing below it can be selected.
	foreign := []string{privTargetTeamWS.String(), privStrangerWS.String(), privStranger.String(), "target personal", "target team"}
	for name, h := range s.callers() {
		t.Run(name, func(t *testing.T) {
			s := s.at(t)
			c := &privacyCheck{g: g}
			for field, call := range calls {
				c.operationKey = call
				f := g.s.Query.Fields.ForName(field)
				o, raw := s.gql(h, "{ "+call+" "+g.selection(f.Type.Name(), 2)+" }")
				require.Nil(t, o.Raw()["errors"], raw)
				c.walk(o.Raw()["data"], "Query", "", "data")
				assert.NotContains(t, raw, privTargetEmail, call)
				assert.NotContains(t, raw, auBilling, call)
				for _, id := range foreign {
					assert.NotContains(t, raw, id, call)
				}
			}
			require.Empty(t, c.leaks)
			require.Positive(t, c.targetSeen, "no selection reached the co-member, so the test proves nothing")
		})
	}
}

func TestActAsUser_Logs(t *testing.T) {
	s := startActAsUserServer(t, false)
	logs := captureLogs(t)
	tok := s.users["Authorization"]

	for _, do := range []func(){
		func() { s.e.GET("/api/users/me").WithHeaders(s.users).Expect().Status(http.StatusOK) },
		func() { s.gql(s.users, `{ me { id email } }`) },
		func() {
			s.e.PATCH("/api/users/me").WithHeaders(s.users).WithJSON(map[string]any{}).Expect().Status(http.StatusForbidden)
		},
	} {
		before := len(logs.String())
		do()
		out := logs.String()[before:]
		var verdicts []string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "\tauth: client call\t") || strings.Contains(l, "\tauth: rejected client call\t") {
				verdicts = append(verdicts, l)
			}
		}
		require.Len(t, verdicts, 1, out)
		entries := requestEntries(out)
		require.Len(t, entries, 1, out)
		for _, l := range []string{verdicts[0], entries[0]} {
			assert.Contains(t, l, `"caller_client_id": "svc-users"`)
			assert.Contains(t, l, `"caller_name": "users-app"`)
			assert.Contains(t, l, `"acting_for": "user:`+auSubject.String()+`"`)
		}
		assert.Contains(t, entries[0], `"viewer_id": "`+auSubject.String()+`"`)
	}
	out := logs.String()
	assert.NotContains(t, out, strings.TrimPrefix(tok, "Bearer "))
	assert.NotContains(t, out, auSubjectEmail)
}
