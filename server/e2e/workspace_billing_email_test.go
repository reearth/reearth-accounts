package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

const teamBillingEmail = "billing@example.com"

var (
	restMemberTeamWID  = id.NewWorkspaceID()
	restForeignTeamWID = id.NewWorkspaceID()
)

func teamWithBilling(wid id.WorkspaceID, alias string, members map[user.ID]workspace.Member) *workspace.Workspace {
	md := workspace.NewMetadata()
	md.SetBillingEmail(teamBillingEmail)
	return workspace.New().ID(wid).Name(alias).Alias(alias).Members(members).Metadata(md).MustBuild()
}

// uId owns the team workspace wId; uId2 is signed in but not a member.
func seedBillingEmailGQL(ctx context.Context, r *repo.Container) error {
	for _, u := range []*user.User{
		user.New().ID(uId).Name("owner").Alias("owner").Email("owner@example.com").
			Auths([]user.Auth{*user.ReearthSub(uId.String())}).Workspace(wId).MustBuild(),
		user.New().ID(uId2).Name("outsider").Alias("outsider").Email("outsider@example.com").
			Auths([]user.Auth{*user.ReearthSub(uId2.String())}).Workspace(wId2).MustBuild(),
	} {
		if err := r.User.Save(ctx, u); err != nil {
			return err
		}
	}
	outsiderWS := workspace.New().ID(wId2).Name("outsider").Alias("outsider").Personal(true).
		Members(map[user.ID]workspace.Member{uId2: {Role: role.RoleOwner, InvitedBy: uId2}}).MustBuild()
	if err := r.Workspace.Save(ctx, outsiderWS); err != nil {
		return err
	}
	return r.Workspace.Save(ctx, teamWithBilling(wId, "team-alias", map[user.ID]workspace.Member{
		uId: {Role: role.RoleOwner, InvitedBy: uId},
	}))
}

var billingEmailQueries = map[string]string{
	"findByID":    `{ r: findByID(id:"` + wId.String() + `") { metadata { billingEmail } } }`,
	"findByIDs":   `{ r: findByIDs(ids:["` + wId.String() + `"]) { metadata { billingEmail } } }`,
	"findByAlias": `{ r: findByAlias(alias:"team-alias") { metadata { billingEmail } } }`,
}

func gqlBillingEmail(t *testing.T, data map[string]any) string {
	t.Helper()
	r := data["r"]
	if list, ok := r.([]any); ok {
		r = list[0]
	}
	return r.(map[string]any)["metadata"].(map[string]any)["billingEmail"].(string)
}

func TestWorkspaceBillingEmail_GQL(t *testing.T) {
	t.Run("anonymous", func(t *testing.T) {
		e, _ := StartServerNoDebug(t, &app.Config{}, false, seedBillingEmailGQL)
		for name, q := range billingEmailQueries {
			t.Run(name, func(t *testing.T) {
				if got := gqlBillingEmail(t, postGQL(t, e, nil, q).Raw()); got != "" {
					t.Fatalf("billingEmail = %q, want empty", got)
				}
			})
		}
	})

	e, _ := StartServer(t, &app.Config{}, false, seedBillingEmailGQL)
	for _, tc := range []struct {
		name   string
		viewer id.UserID
		want   string
	}{
		{"signed-in non-member", uId2, ""},
		{"member", uId, teamBillingEmail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, q := range billingEmailQueries {
				t.Run(name, func(t *testing.T) {
					// A query of only bypassed fields skips auth, so add me to sign in.
					q := strings.Replace(q, "{ r:", "{ me { id } r:", 1)
					if got := gqlBillingEmail(t, postGQL(t, e, &tc.viewer, q).Raw()); got != tc.want {
						t.Fatalf("billingEmail = %q, want %q", got, tc.want)
					}
				})
			}
		})
	}
}

// The mock-auth caller (restDemoUID) is a reader of restMemberTeamWID and not
// a member of restForeignTeamWID.
func seedBillingEmailREST(maintainer bool) Seeder {
	return func(ctx context.Context, r *repo.Container) error {
		if err := seedDemoUser(ctx, r); err != nil {
			return err
		}
		for _, w := range []*workspace.Workspace{
			teamWithBilling(restMemberTeamWID, "member-team", map[user.ID]workspace.Member{
				restOtherUID: {Role: role.RoleOwner, InvitedBy: restOtherUID},
				restDemoUID:  {Role: role.RoleReader, InvitedBy: restOtherUID},
			}),
			teamWithBilling(restForeignTeamWID, "foreign-team", map[user.ID]workspace.Member{
				restOtherUID: {Role: role.RoleOwner, InvitedBy: restOtherUID},
			}),
		} {
			if err := r.Workspace.Save(ctx, w); err != nil {
				return err
			}
		}
		if !maintainer {
			return nil
		}
		rl := role.New().NewID().Name(string(role.RoleMaintainer)).MustBuild()
		if err := r.Role.Save(ctx, *rl); err != nil {
			return err
		}
		p, err := permittable.New().NewID().UserID(restDemoUID).RoleIDs([]id.RoleID{rl.ID()}).Build()
		if err != nil {
			return err
		}
		return r.Permittable.Save(ctx, *p)
	}
}

func TestWorkspaceBillingEmail_REST(t *testing.T) {
	exp, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seedBillingEmailREST(false))

	t.Run("non-member gets empty", func(t *testing.T) {
		exp.GET("/api/workspaces/"+restForeignTeamWID.String()).Expect().Status(http.StatusOK).
			JSON().Object().Value("metadata").Object().HasValue("billing_email", "")
		exp.GET("/api/workspaces").WithQuery("alias", "foreign-team").Expect().Status(http.StatusOK).
			JSON().Object().Value("metadata").Object().HasValue("billing_email", "")
	})

	t.Run("member gets it", func(t *testing.T) {
		exp.GET("/api/workspaces/"+restMemberTeamWID.String()).Expect().Status(http.StatusOK).
			JSON().Object().Value("metadata").Object().HasValue("billing_email", teamBillingEmail)
	})
}

func TestWorkspaceBillingEmail_RESTListAllForMaintainer(t *testing.T) {
	exp, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seedBillingEmailREST(true))

	items := exp.GET("/api/workspaces/all").Expect().Status(http.StatusOK).
		JSON().Object().Value("items").Array()
	for _, v := range items.Iter() {
		if o := v.Object(); o.Value("id").String().Raw() == restForeignTeamWID.String() {
			o.Value("metadata").Object().HasValue("billing_email", teamBillingEmail)
			return
		}
	}
	t.Fatalf("workspace %s not in /api/workspaces/all: %v", restForeignTeamWID, items.Raw())
}
