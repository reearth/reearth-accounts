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
	withPhoto *workspace.Workspace
	noPhoto   *workspace.Workspace
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
	return e, photoFixture{me: me, withPhoto: withPhoto, noPhoto: noPhoto}
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

func TestREST_MeSigningErrorFails(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "users/me.png").Return("", errors.New("gcs down"))
	e, _ := newPhotoEcho(t, storage, "users/me.png")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/users/me", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "users/me.png")
}

func TestREST_OtherMeRoutesReturnStoredPhotoURL(t *testing.T) {
	routes := []struct {
		method, path, body, want string
	}{
		{http.MethodPatch, "/api/users/me", `{"photo_url":"users/new.png"}`, "users/new.png"},
		{http.MethodDelete, "/api/users/me/auths/auth0", "", "users/me.png"},
		{http.MethodPost, "/api/auth/logout", "", "users/me.png"},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			storage := gatewaymock.NewMockStorage(gomock.NewController(t))
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
	getJSON(t, e, "/api/workspaces?ids="+f.withPhoto.ID().String()+","+f.noPhoto.ID().String(), &res)
	assert.Equal(t, map[string]string{
		f.withPhoto.ID().String(): "https://signed/a",
		f.noPhoto.ID().String():   "",
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
		f.noPhoto.ID().String():   "",
	}, photosByID(res))
}

func TestREST_UpdateMeRejectsAbsolutePhotoURL(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	e, _ := newPhotoEcho(t, storage, "users/me.png")

	req := httptest.NewRequest(http.MethodPatch, "/api/users/me", strings.NewReader(`{"photo_url":"https://example.com/photo.jpg"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
