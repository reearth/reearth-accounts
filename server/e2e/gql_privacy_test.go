package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
)

// ADR-0003 decision 8: every query, run anonymously and as a signed-in user
// who shares no workspace with the target, must not return any of the
// target's SELF or WORKSPACE fields. Queries and selections come from the
// schema, so a new query or field is covered without editing this test.

var (
	privTarget       = id.NewUserID()
	privTargetWS     = id.NewWorkspaceID()
	privTargetTeamWS = id.NewWorkspaceID()
	privStranger     = id.NewUserID()
	privStrangerWS   = id.NewWorkspaceID()
)

const (
	privTargetEmail = "privacy-target@example.com"
	privTargetAlias = "privacy-target"
	privTargetName  = "Privacy Target"
	maxPrivDepth    = 8
)

func seedPrivacy(ctx context.Context, r *repo.Container) error {
	md := user.NewMetadata()
	md.LangFrom("ja")
	md.SetDescription("target description")
	md.SetWebsite("https://target.example.com")
	md.SetPhotoURL("https://target.example.com/photo.png")
	md.SetTheme(user.ThemeDark)
	target := user.New().ID(privTarget).Name(privTargetName).Alias(privTargetAlias).Email(privTargetEmail).
		Metadata(md).Auths([]user.Auth{*user.ReearthSub(privTarget.String())}).Workspace(privTargetWS).MustBuild()
	stranger := user.New().ID(privStranger).Name("stranger").Alias("stranger").Email("stranger@example.com").
		Auths([]user.Auth{*user.ReearthSub(privStranger.String())}).Workspace(privStrangerWS).MustBuild()
	for _, u := range []*user.User{target, stranger} {
		if err := r.User.Save(ctx, u); err != nil {
			return err
		}
	}
	member := func(u user.ID) map[idx.ID[id.User]]workspace.Member {
		return map[idx.ID[id.User]]workspace.Member{u: {Role: role.RoleOwner, InvitedBy: u}}
	}
	for _, w := range []*workspace.Workspace{
		workspace.New().ID(privTargetWS).Name("target personal").Alias("privacy-target-ws").Personal(true).Members(member(privTarget)).MustBuild(),
		workspace.New().ID(privTargetTeamWS).Name("target team").Alias("privacy-target-team").Members(member(privTarget)).MustBuild(),
		workspace.New().ID(privStrangerWS).Name("stranger personal").Alias("stranger-ws").Personal(true).Members(member(privStranger)).MustBuild(),
	} {
		if err := r.Workspace.Save(ctx, w); err != nil {
			return err
		}
	}
	return nil
}

type privacyGen struct{ s *ast.Schema }

func (g privacyGen) candidates(t *ast.Type) []string {
	if t.Elem != nil {
		var out []string
		for _, c := range g.candidates(t.Elem) {
			out = append(out, "["+c+"]")
		}
		return out
	}
	q := func(vs ...string) []string {
		out := make([]string, len(vs))
		for i, v := range vs {
			b, _ := json.Marshal(v)
			out[i] = string(b)
		}
		return out
	}
	switch t.NamedType {
	case "ID":
		return q(privTarget.String(), privTargetWS.String(), privTargetTeamWS.String())
	case "String":
		return q(privTargetAlias, privTargetEmail, privTargetName)
	case "Int":
		return []string{"1"}
	case "Boolean":
		return []string{"false"}
	}
	def := g.s.Types[t.NamedType]
	switch def.Kind {
	case ast.Enum:
		var out []string
		for _, v := range def.EnumValues {
			out = append(out, v.Name)
		}
		return out
	case ast.InputObject:
		var parts []string
		for _, f := range def.Fields {
			if f.Type.NonNull {
				parts = append(parts, f.Name+": "+g.candidates(f.Type)[0])
			}
		}
		return []string{"{" + strings.Join(parts, ", ") + "}"}
	}
	return q("x")
}

// calls returns one field call per combination of required-argument values.
func (g privacyGen) calls(f *ast.FieldDefinition) []string {
	calls := []string{""}
	for _, a := range f.Arguments {
		if !a.Type.NonNull {
			continue
		}
		var next []string
		for _, prefix := range calls {
			for _, c := range g.candidates(a.Type) {
				next = append(next, prefix+a.Name+": "+c+", ")
			}
		}
		calls = next
	}
	for i, c := range calls {
		if c == "" {
			calls[i] = f.Name
		} else {
			calls[i] = f.Name + "(" + strings.TrimSuffix(c, ", ") + ")"
		}
	}
	return calls
}

func (g privacyGen) selection(typeName string, depth int) string {
	def := g.s.Types[typeName]
	switch def.Kind {
	case ast.Scalar, ast.Enum:
		return ""
	case ast.Union, ast.Interface:
		parts := []string{"__typename"}
		for _, p := range g.s.GetPossibleTypes(def) {
			parts = append(parts, "... on "+p.Name+" "+g.selection(p.Name, depth))
		}
		return "{ " + strings.Join(parts, " ") + " }"
	}
	parts := []string{"__typename"}
	for _, f := range def.Fields {
		if strings.HasPrefix(f.Name, "__") || hasRequiredArgs(f) {
			continue
		}
		sub := g.selection(f.Type.Name(), depth+1)
		if sub != "" && depth >= maxPrivDepth {
			continue
		}
		parts = append(parts, f.Name+" "+sub)
	}
	return "{ " + strings.Join(parts, " ") + " }"
}

func hasRequiredArgs(f *ast.FieldDefinition) bool {
	for _, a := range f.Arguments {
		if a.Type.NonNull && a.DefaultValue == nil {
			return true
		}
	}
	return false
}

func visibilityLevel(f *ast.FieldDefinition) string {
	d := f.Directives.ForName("visibility")
	if d == nil {
		return "PUBLIC"
	}
	return d.Arguments.ForName("level").Value.Raw
}

// ownerKeys name the field that holds the user an object describes; the
// server's @visibility resolver uses the same rule.
var ownerKeys = map[string]string{"User": "id", "Me": "id", "WorkspaceUserMember": "userId"}

type privacyCheck struct {
	g            privacyGen
	targetSeen   int
	leaks        []string
	operationKey string
}

func (c *privacyCheck) walk(v any, typeName, owner, path string) {
	switch val := v.(type) {
	case []any:
		for i, item := range val {
			c.walk(item, typeName, owner, fmt.Sprintf("%s[%d]", path, i))
		}
	case map[string]any:
		typ, _ := val["__typename"].(string)
		if typ == "" {
			typ = typeName
		}
		def := c.g.s.Types[typ]
		if def == nil {
			return
		}
		if k, ok := ownerKeys[typ]; ok {
			owner, _ = val[k].(string)
		}
		if owner == privTarget.String() {
			c.targetSeen++
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			f := def.Fields.ForName(k)
			if f == nil {
				continue
			}
			if level := visibilityLevel(f); level != "PUBLIC" && owner == privTarget.String() && !isEmpty(val[k]) {
				c.leaks = append(c.leaks, fmt.Sprintf("%s: %s.%s (%s) = %v", c.operationKey, path, k, level, val[k]))
			}
			c.walk(val[k], f.Type.Name(), owner, path+"."+k)
		}
	}
}

func isEmpty(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case bool:
		return !val
	case float64:
		return val == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		for k, x := range val {
			if k != "__typename" && !isEmpty(x) {
				return false
			}
		}
		return true
	}
	return false
}

func TestGQLPrivacy_NoTargetPrivateFieldsForOthers(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, false, seedPrivacy)
	g := privacyGen{s: gql.NewExecutableSchema(gql.Config{}).Schema()}

	viewers := map[string]func(*httpexpect.Request) *httpexpect.Request{
		"anonymous": func(r *httpexpect.Request) *httpexpect.Request { return r },
		"stranger": func(r *httpexpect.Request) *httpexpect.Request {
			return r.WithHeader("Authorization", "Bearer test").WithHeader("X-Reearth-Debug-User", privStranger.String())
		},
	}
	for name, as := range viewers {
		t.Run(name, func(t *testing.T) {
			c := &privacyCheck{g: g}
			ops := 0
			for _, f := range g.s.Query.Fields {
				if strings.HasPrefix(f.Name, "__") {
					continue
				}
				sel := g.selection(f.Type.Name(), 2)
				for _, call := range g.calls(f) {
					ops++
					c.operationKey = call
					res := as(e.POST("/api/graphql")).WithJSON(GraphQLRequest{Query: "{ " + call + " " + sel + " }"}).Expect()
					status := res.Raw().StatusCode
					if status == http.StatusUnauthorized {
						continue
					}
					require.Equal(t, http.StatusOK, status, call)
					var body map[string]any
					require.NoError(t, json.Unmarshal([]byte(res.Body().Raw()), &body))
					c.walk(body["data"], "Query", "", "data")
				}
			}
			require.Empty(t, c.leaks)
			require.Greater(t, ops, 30)
			require.Positive(t, c.targetSeen, "no query returned the target, so the test proves nothing")
			t.Logf("%d operations, target seen in %d objects", ops, c.targetSeen)
		})
	}
}
