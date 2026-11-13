package http_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type photoFixture struct {
	me        *user.User
	other     *user.User
	noPhoto   *user.User
	withPhoto *workspace.Workspace
	noPhotoWS *workspace.Workspace
}

func newPhotoEcho(t *testing.T, storage gateway.Storage, mePhoto string) (*echo.Echo, photoFixture) {
	t.Helper()
	db := accountmemory.New()

	uid := user.NewID()
	personal := workspace.NewID()
	me := user.New().ID(uid).Name("me").Email("me@example.com").
		Workspace(personal).
		Metadata(user.MetadataFrom(mePhoto, "", "", user.NewMetadata().Lang(), user.ThemeDefault)).
		MustBuild()
	require.NoError(t, db.User.Save(t.Context(), me))
	other := user.New().NewID().Name("other").Email("other@example.com").Workspace(workspace.NewID()).
		Metadata(user.MetadataFrom("users/other.png", "", "", user.NewMetadata().Lang(), user.ThemeDefault)).
		MustBuild()
	require.NoError(t, db.User.Save(t.Context(), other))
	noPhotoUser := user.New().NewID().Name("plain").Email("plain@example.com").Workspace(workspace.NewID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), noPhotoUser))

	members := map[user.ID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}
	withPhoto := workspace.New().ID(personal).Name("a").Alias("a").Members(members).
		Metadata(workspace.MetadataFrom("", "", "", "", "workspaces/a.png")).MustBuild()
	noPhoto := workspace.New().NewID().Name("b").Alias("b").Members(members).MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), withPhoto))
	require.NoError(t, db.Workspace.Save(t.Context(), noPhoto))

	uc := &interfaces.Container{
		User:      interactor.NewUser(db, nil, nil, "", ""),
		Workspace: interactor.NewWorkspace(db, nil, nil),
	}
	op := &workspace.Operator{
		User:             &uid,
		OwningWorkspaces: id.WorkspaceIDList{withPhoto.ID(), noPhoto.ID()},
	}
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
	return e, photoFixture{me: me, other: other, noPhoto: noPhotoUser, withPhoto: withPhoto, noPhotoWS: noPhoto}
}

func getJSON(t *testing.T, e *echo.Echo, path string, out any) {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
}

func photosByID(ws []httpmodel.WorkspaceResponse) map[string]string {
	out := map[string]string{}
	for _, w := range ws {
		out[w.ID] = w.Metadata.PhotoURL
	}
	return out
}

func TestREST_MePhotoURLIsSigned(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "users/me.png").Return("https://signed/me", nil)
	e, _ := newPhotoEcho(t, storage, "users/me.png")

	var res httpmodel.MeResponse
	getJSON(t, e, "/api/users/me", &res)
	assert.Equal(t, "https://signed/me", res.Metadata.PhotoURL)
}

func TestREST_MeEmptyPhotoURLIsNotSigned(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	e, _ := newPhotoEcho(t, storage, "")

	var res httpmodel.MeResponse
	getJSON(t, e, "/api/users/me", &res)
	assert.Empty(t, res.Metadata.PhotoURL)
}

func TestREST_MeSigningErrorEmptiesPhotoURL(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "users/me.png").Return("", errors.New("gcs down"))
	e, _ := newPhotoEcho(t, storage, "users/me.png")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/me", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "users/me.png")
	var res httpmodel.MeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Empty(t, res.Metadata.PhotoURL)
}

func TestREST_OtherMeRoutesSignPhotoURL(t *testing.T) {
	routes := []struct {
		method, path, body, want string
	}{
		{http.MethodPatch, "/api/users/me", `{"photo_url":"users/new.png"}`, "https://signed/new"},
		{http.MethodDelete, "/api/users/me/auths/auth0", "", "https://signed/me"},
		{http.MethodPost, "/api/auth/logout", "", "https://signed/me"},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			storage := gatewaymock.NewMockStorage(gomock.NewController(t))
			storage.EXPECT().GetSignedURL(gomock.Any(), "users/me.png").Return("https://signed/me", nil).AnyTimes()
			storage.EXPECT().GetSignedURL(gomock.Any(), "users/new.png").Return("https://signed/new", nil).AnyTimes()
			e, _ := newPhotoEcho(t, storage, "users/me.png")

			req := httptest.NewRequest(r.method, r.path, strings.NewReader(r.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var res httpmodel.MeResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
			assert.Equal(t, r.want, res.Metadata.PhotoURL)
		})
	}
}

func TestREST_WorkspacePhotoURLIsSigned(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/a.png").Return("https://signed/a", nil)
	e, f := newPhotoEcho(t, storage, "")

	var res httpmodel.WorkspaceResponse
	getJSON(t, e, "/api/workspaces/"+f.withPhoto.ID().String(), &res)
	assert.Equal(t, "https://signed/a", res.Metadata.PhotoURL)
}

func TestREST_WorkspaceListPhotoURLsAreSigned(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/a.png").Return("https://signed/a", nil).Times(1)
	e, f := newPhotoEcho(t, storage, "")

	var res []httpmodel.WorkspaceResponse
	getJSON(t, e, "/api/workspaces?ids="+f.withPhoto.ID().String()+","+f.noPhotoWS.ID().String(), &res)
	assert.Equal(t, map[string]string{
		f.withPhoto.ID().String(): "https://signed/a",
		f.noPhotoWS.ID().String(): "",
	}, photosByID(res))
}

func TestREST_WorkspaceSigningErrorEmptiesPhotoURL(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/a.png").Return("", errors.New("gcs down"))
	e, f := newPhotoEcho(t, storage, "")

	var res []httpmodel.WorkspaceResponse
	getJSON(t, e, "/api/workspaces?user_id="+f.me.ID().String(), &res)
	assert.Equal(t, map[string]string{
		f.withPhoto.ID().String(): "",
		f.noPhotoWS.ID().String(): "",
	}, photosByID(res))
}

func TestREST_UserLookupsSignPhotoURL(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "users/other.png").Return("https://signed/other", nil).AnyTimes()
	e, f := newPhotoEcho(t, storage, "")

	var one httpmodel.UserResponse
	getJSON(t, e, "/api/users/"+f.other.ID().String(), &one)
	assert.Equal(t, "https://signed/other", one.Metadata.PhotoURL)

	var list []httpmodel.UserResponse
	getJSON(t, e, "/api/users?ids="+f.other.ID().String()+","+f.noPhoto.ID().String(), &list)
	got := map[string]string{}
	for _, u := range list {
		got[u.ID] = u.Metadata.PhotoURL
	}
	assert.Equal(t, map[string]string{
		f.other.ID().String():   "https://signed/other",
		f.noPhoto.ID().String(): "",
	}, got)
}
