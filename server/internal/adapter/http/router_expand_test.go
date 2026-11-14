package http_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/mock/gomock"
)

type countingUserUsecase struct {
	interfaces.User
	fetchByID int
}

func (u *countingUserUsecase) FetchByID(ctx context.Context, ids user.IDList) (user.List, error) {
	u.fetchByID++
	return u.User.FetchByID(ctx, ids)
}

type expandFixture struct {
	e        *echo.Echo
	db       *repo.Container
	users    *countingUserUsecase
	me       *user.User
	other    *user.User
	gone     *user.User
	personal *workspace.Workspace
	team     *workspace.Workspace
}

// newExpandEcho seeds a personal workspace with a photo and a team workspace.
// Both list me, other (with a stored photo, a host and a verification), gone
// (soft-deleted), and a member whose user no longer exists.
func newExpandEcho(t *testing.T, savePersonal bool) expandFixture {
	t.Helper()
	db := accountmemory.New()

	personalID := workspace.NewID()
	me := user.New().NewID().Name("me").Email("me@example.com").Workspace(personalID).MustBuild()
	other := user.New().NewID().Name("other").Email("other@example.com").Workspace(workspace.NewID()).
		Metadata(user.MetadataFrom("users/other.png", "", "", user.NewMetadata().Lang(), user.ThemeDefault)).
		Verification(user.VerificationFrom("code", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), true)).
		MustBuild().WithHost("other.example.com")
	deletedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	gone := user.New().NewID().Name("gone").Email("gone@example.com").Workspace(workspace.NewID()).DeletedAt(&deletedAt).MustBuild()
	for _, u := range []*user.User{me, other, gone} {
		require.NoError(t, db.User.Save(t.Context(), u))
	}

	members := map[user.ID]workspace.Member{
		me.ID():      {Role: role.RoleOwner, InvitedBy: me.ID()},
		other.ID():   {Role: role.RoleReader, InvitedBy: me.ID()},
		gone.ID():    {Role: role.RoleReader, InvitedBy: me.ID()},
		user.NewID(): {Role: role.RoleReader, InvitedBy: me.ID()},
	}
	personal := workspace.New().ID(personalID).Name("p").Alias("p").Personal(true).Members(members).
		Metadata(workspace.MetadataFrom("", "", "", "", "workspaces/p.png")).MustBuild()
	team := workspace.New().NewID().Name("t").Alias("t").Members(members).MustBuild()
	if savePersonal {
		require.NoError(t, db.Workspace.Save(t.Context(), personal))
	}
	require.NoError(t, db.Workspace.Save(t.Context(), team))

	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/p.png").Return("https://signed/p", nil).AnyTimes()
	storage.EXPECT().GetSignedURL(gomock.Any(), "users/other.png").Return("https://signed/other", nil).AnyTimes()

	users := &countingUserUsecase{User: interactor.NewUser(db, nil, nil, "", "")}
	uc := &interfaces.Container{User: users, Workspace: interactor.NewWorkspace(db, nil, nil)}
	uid := me.ID()
	op := &workspace.Operator{User: &uid, OwningWorkspaces: id.WorkspaceIDList{personal.ID(), team.ID()}}
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		AuthResolver: func(echo.Context, *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
			return me, op, nil
		},
		UsecaseMiddleware: func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.SetRequest(c.Request().WithContext(adapter.AttachUsecases(c.Request().Context(), uc)))
				return next(c)
			}
		},
		Photos: photoSigner(t, storage),
	})
	return expandFixture{e: e, db: db, users: users, me: me, other: other, gone: gone, personal: personal, team: team}
}

func (f expandFixture) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	return rec
}

func (f expandFixture) get(t *testing.T, path string) []byte {
	t.Helper()
	rec := f.do(t, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.Bytes()
}

// sortedMembers decodes a workspace with its members sorted by user id, since
// members come from a map and their order varies between responses.
func sortedMembers(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var w map[string]any
	require.NoError(t, json.Unmarshal(raw, &w))
	members, _ := w["members"].([]any)
	slices.SortFunc(members, func(a, b any) int {
		return strings.Compare(a.(map[string]any)["user_id"].(string), b.(map[string]any)["user_id"].(string))
	})
	return w
}

func memberIDs(ms []httpmodel.WorkspaceMemberResponse) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.UserID)
	}
	return out
}

// graphqlUserFields lists the fields of the GraphQL User type, read from the
// schema. Every one is a single word, so REST spells it the same.
func graphqlUserFields(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../../../schemas/*.graphql")
	require.NoError(t, err)
	var sources []*ast.Source
	for _, p := range paths {
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		sources = append(sources, &ast.Source{Name: p, Input: string(b)})
	}
	schema, gqlErr := gqlparser.LoadSchema(sources...)
	require.Nil(t, gqlErr)
	var fields []string
	for _, f := range schema.Types["User"].Fields {
		fields = append(fields, f.Name)
	}
	return fields
}

// assertEmbeddedUsers checks every member carries its user with exactly the
// GraphQL User fields: only the caller's email, the signed photo URL, and a
// soft-deleted user shown like a live one.
func assertEmbeddedUsers(t *testing.T, f expandFixture, w map[string]json.RawMessage) {
	t.Helper()
	var members []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w["members"], &members))
	byID := map[string]map[string]any{}
	for _, m := range members {
		require.Contains(t, m, "user")
		var u map[string]any
		require.NoError(t, json.Unmarshal(m["user"], &u))
		byID[u["id"].(string)] = u
	}
	require.Len(t, byID, 3)

	other := byID[f.other.ID().String()]
	assert.ElementsMatch(t, graphqlUserFields(t), slices.Collect(maps.Keys(other)))
	assert.Equal(t, "", other["email"])
	assert.Equal(t, "https://signed/other", other["metadata"].(map[string]any)["photo_url"])

	assert.Equal(t, "me@example.com", byID[f.me.ID().String()]["email"])

	gone := byID[f.gone.ID().String()]
	assert.Equal(t, map[string]any{
		"id":        f.gone.ID().String(),
		"name":      "gone",
		"alias":     "",
		"email":     "",
		"workspace": f.gone.Workspace().String(),
		"auths":     nil,
		"metadata": map[string]any{
			"description": "", "website": "", "photo_url": "", "lang": "und", "theme": "default",
		},
	}, gone)
}

func TestREST_ExpandAbsentAddsNothing(t *testing.T) {
	f := newExpandEcho(t, true)

	var me map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(f.get(t, "/api/users/me"), &me))
	assert.NotContains(t, me, "my_workspace")

	var w struct {
		Members []map[string]json.RawMessage `json:"members"`
	}
	require.NoError(t, json.Unmarshal(f.get(t, "/api/workspaces/"+f.team.ID().String()), &w))
	require.Len(t, w.Members, 3)
	for _, m := range w.Members {
		assert.NotContains(t, m, "user")
	}
}

func TestREST_ExpandMyWorkspace(t *testing.T) {
	f := newExpandEcho(t, true)

	var me struct {
		MyWorkspace json.RawMessage `json:"my_workspace"`
	}
	require.NoError(t, json.Unmarshal(f.get(t, "/api/users/me?expand=my_workspace"), &me))
	direct := f.get(t, "/api/workspaces/"+f.personal.ID().String())
	assert.Equal(t, sortedMembers(t, direct), sortedMembers(t, me.MyWorkspace))

	var w httpmodel.WorkspaceResponse
	require.NoError(t, json.Unmarshal(me.MyWorkspace, &w))
	assert.Equal(t, f.personal.ID().String(), w.ID)
	assert.Equal(t, "https://signed/p", w.Metadata.PhotoURL)
	assert.ElementsMatch(t, []string{f.me.ID().String(), f.other.ID().String(), f.gone.ID().String()}, memberIDs(w.Members))
}

func TestREST_ExpandMyWorkspaceMissingOmitsIt(t *testing.T) {
	f := newExpandEcho(t, false)

	var me map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(f.get(t, "/api/users/me?expand=my_workspace"), &me))
	assert.NotContains(t, me, "my_workspace")
	assert.Contains(t, me, "id")
}

func TestREST_ExpandMemberUsers(t *testing.T) {
	f := newExpandEcho(t, true)

	t.Run("get", func(t *testing.T) {
		var w map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(f.get(t, "/api/workspaces/"+f.team.ID().String()+"?expand=members.user"), &w))
		assertEmbeddedUsers(t, f, w)
	})

	t.Run("list by user", func(t *testing.T) {
		var ws []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(f.get(t, "/api/workspaces?user_id="+f.me.ID().String()+"&expand=members.user"), &ws))
		require.Len(t, ws, 2)
		for _, w := range ws {
			assertEmbeddedUsers(t, f, w)
		}
	})

	t.Run("mutation", func(t *testing.T) {
		rec := f.do(t, http.MethodPatch, "/api/workspaces/"+f.team.ID().String()+"?expand=members.user", `{"name":"renamed"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var w map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
		assertEmbeddedUsers(t, f, w)
	})
}

func TestREST_ExpandListLooksUpUsersOnce(t *testing.T) {
	f := newExpandEcho(t, true)

	f.get(t, "/api/workspaces?user_id="+f.me.ID().String()+"&expand=members.user")
	assert.Equal(t, 1, f.users.fetchByID)
}

func TestREST_ExpandUnknownValueIsRejected(t *testing.T) {
	f := newExpandEcho(t, true)
	wsPath := "/api/workspaces/" + f.team.ID().String()

	cases := []struct {
		name, method, path, body, allowed string
	}{
		{"me rejects members.user", http.MethodGet, "/api/users/me?expand=my_workspace,members.user", "", "my_workspace"},
		{"workspace rejects my_workspace", http.MethodGet, wsPath + "?expand=my_workspace", "", "members.user"},
		{"list rejects unknown", http.MethodGet, "/api/workspaces?user_id=" + f.me.ID().String() + "&expand=bogus", "", "members.user"},
		{"mutation rejects before writing", http.MethodPatch, wsPath + "?expand=bogus", `{"name":"renamed"}`, "members.user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.do(t, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var res struct {
				FieldErrors []struct {
					Field   string `json:"field"`
					Message string `json:"message"`
				} `json:"field_errors"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
			require.Len(t, res.FieldErrors, 1)
			assert.Equal(t, "expand", res.FieldErrors[0].Field)
			assert.Equal(t, "must be one of: "+tc.allowed, res.FieldErrors[0].Message)
		})
	}

	stored, err := f.db.Workspace.FindByID(t.Context(), f.team.ID())
	require.NoError(t, err)
	assert.Equal(t, "t", stored.Name())
}

func TestREST_ExpandEmptySegmentsAreIgnored(t *testing.T) {
	f := newExpandEcho(t, true)
	wsPath := "/api/workspaces/" + f.team.ID().String()

	for _, q := range []string{"?expand=", "?expand=,", "?expand=%20"} {
		var w struct {
			Members []map[string]json.RawMessage `json:"members"`
		}
		require.NoError(t, json.Unmarshal(f.get(t, wsPath+q), &w), q)
		require.Len(t, w.Members, 3, q)
		for _, m := range w.Members {
			assert.NotContains(t, m, "user", q)
		}
	}

	var w map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(f.get(t, wsPath+"?expand=members.user,"), &w))
	assertEmbeddedUsers(t, f, w)
}
