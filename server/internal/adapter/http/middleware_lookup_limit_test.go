package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestUserLookupLimit_CountsAClientCallAgainstTheClient(t *testing.T) {
	const iss = "https://idp.example/"
	iid := id.NewIntegrationID()
	anonymous := func(clientID string) context.Context {
		return (&adapter.ClientCall{Issuer: iss, Client: &workspace.Client{ID: clientID, Name: clientID}}).Attach(context.Background())
	}
	forIntegration := func(clientID string) context.Context {
		c := &workspace.Client{ID: clientID, Name: clientID}
		return (&adapter.ClientCall{
			Issuer: iss, Client: c, Subject: workspace.IntegrationSubject(iid),
			Operator: &workspace.Operator{Integration: &iid, Client: c},
		}).Attach(context.Background())
	}
	plainUser := func(u *user.User) context.Context {
		uid := u.ID()
		return adapter.AttachOperator(adapter.AttachUser(context.Background(), u), &workspace.Operator{User: &uid})
	}
	alice := user.New().NewID().Name("alice").Email("alice@example.com").MustBuild()
	bob := user.New().NewID().Name("bob").Email("bob@example.com").MustBuild()

	type call struct {
		ctx context.Context
		ip  string
	}
	cases := map[string]struct {
		first, second call
		shared        bool
	}{
		"one anonymous client from two addresses":   {call{anonymous("svc-a"), "192.0.2.1"}, call{anonymous("svc-a"), "192.0.2.2"}, true},
		"two anonymous clients":                     {call{anonymous("svc-a"), "192.0.2.1"}, call{anonymous("svc-b"), "192.0.2.1"}, false},
		"one integration client from two addresses": {call{forIntegration("svc-a"), "192.0.2.1"}, call{forIntegration("svc-a"), "192.0.2.2"}, true},
		"two integration clients":                   {call{forIntegration("svc-a"), "192.0.2.1"}, call{forIntegration("svc-b"), "192.0.2.1"}, false},
		"one user from two addresses":               {call{plainUser(alice), "192.0.2.1"}, call{plainUser(alice), "192.0.2.2"}, true},
		"two users from one address":                {call{plainUser(alice), "192.0.2.1"}, call{plainUser(bob), "192.0.2.1"}, false},
		"no token from one address":                 {call{context.Background(), "192.0.2.1"}, call{context.Background(), "192.0.2.1"}, true},
		"no token from two addresses":               {call{context.Background(), "192.0.2.1"}, call{context.Background(), "192.0.2.2"}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mw := UserLookupLimit(ratelimit.New(ratelimit.Config{Enforce: true, PerMinute: 1, Burst: 1}))
			do := func(c call) int {
				req := httptest.NewRequest(http.MethodGet, "/api/users/search", nil).WithContext(c.ctx)
				req.RemoteAddr = c.ip + ":1234"
				rec := httptest.NewRecorder()
				ec := echo.New().NewContext(req, rec)
				if err := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })(ec); err != nil {
					httpinternal.CustomHTTPErrorHandler(err, ec)
				}
				return rec.Code
			}
			assert.Equal(t, http.StatusOK, do(tc.first))
			want := http.StatusOK
			if tc.shared {
				want = http.StatusTooManyRequests
			}
			assert.Equal(t, want, do(tc.second))
		})
	}
}
