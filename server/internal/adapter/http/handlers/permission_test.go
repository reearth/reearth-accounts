package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/handlers"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

type allowAll struct{}

func (allowAll) CheckPermission(context.Context, user.ID, interfaces.CheckPermissionParam) (*interfaces.CheckPermissionResult, error) {
	return &interfaces.CheckPermissionResult{Allowed: true}, nil
}

func TestPermissionCheck_RefusesClientCalls(t *testing.T) {
	u := user.New().NewID().Name("u").Email("u@example.com").MustBuild()
	uid := u.ID()
	iid := id.NewIntegrationID()
	svc := &workspace.Client{ID: "svc", Name: "svc"}
	bg := adapter.AttachUsecases(context.Background(), &interfaces.Container{Cerbos: allowAll{}})
	call := func(ctx context.Context) int {
		req := httptest.NewRequest(http.MethodPost, "/api/permissions/check",
			strings.NewReader(`{"service":"s","resource":"r","action":"read"}`)).WithContext(ctx)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		if err := handlers.NewPermissionHandler().Check(c); err != nil {
			httpinternal.CustomHTTPErrorHandler(err, c)
		}
		return rec.Code
	}
	actingForUser := (&adapter.ClientCall{
		Client: svc, Subject: workspace.UserSubject(uid), User: u,
		Operator: &workspace.Operator{User: &uid, Client: svc},
	}).Attach(bg)
	actingForIntegration := (&adapter.ClientCall{
		Client: svc, Subject: workspace.IntegrationSubject(iid),
		Operator: &workspace.Operator{Integration: &iid, Client: svc},
	}).Attach(bg)
	plainUser := adapter.AttachOperator(adapter.AttachUser(bg, u), &workspace.Operator{User: &uid})

	assert.Equal(t, http.StatusForbidden, call(actingForUser), "a client acting for a user")
	assert.Equal(t, http.StatusForbidden, call(actingForIntegration), "a client acting for an integration")
	assert.Equal(t, http.StatusOK, call(plainUser), "a user's own token")
}
