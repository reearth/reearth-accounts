package e2e

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
)

func TestREST_UserEmailHiddenFromOtherUsers(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)

	other := restOtherUID.String()
	ids := other + "," + restDemoUID.String()
	lookups := map[string]func() *httpexpect.Object{
		"get by id": func() *httpexpect.Object {
			return exp.GET("/api/users/" + other).Expect().Status(http.StatusOK).JSON().Object()
		},
		"list by ids": func() *httpexpect.Object {
			return findByID(t, exp.GET("/api/users").WithQuery("ids", ids).
				Expect().Status(http.StatusOK).JSON().Array(), other)
		},
		"list by ids paginated": func() *httpexpect.Object {
			return findByID(t, exp.GET("/api/users").WithQuery("ids", ids).WithQuery("page", 1).
				Expect().Status(http.StatusOK).JSON().Object().Value("items").Array(), other)
		},
		"search": func() *httpexpect.Object {
			return findByID(t, exp.GET("/api/users/search").WithQuery("keyword", "Other REST").
				Expect().Status(http.StatusOK).JSON().Array(), other)
		},
		"by alias": func() *httpexpect.Object {
			return exp.GET("/api/users/by-alias").WithQuery("alias", "other-rest-user").
				Expect().Status(http.StatusOK).JSON().Object()
		},
		"by name or alias": func() *httpexpect.Object {
			return findByID(t, exp.GET("/api/users/by-name-or-alias").WithQuery("q", "other-rest-user").
				Expect().Status(http.StatusOK).JSON().Array(), other)
		},
		"by name or email": func() *httpexpect.Object {
			return exp.GET("/api/users/by-name-or-email").WithQuery("q", "other-rest@example.com").
				Expect().Status(http.StatusOK).JSON().Object()
		},
	}
	for name, lookup := range lookups {
		t.Run(name, func(t *testing.T) {
			lookup().HasValue("id", other).HasValue("email", "")
		})
	}

	t.Run("listing all users with every email is maintainer only", func(t *testing.T) {
		exp.GET("/api/users/all").Expect().Status(http.StatusForbidden).
			JSON().Object().NotContainsKey("items")
	})

	t.Run("own record keeps email", func(t *testing.T) {
		exp.GET("/api/users/"+restDemoUID.String()).Expect().Status(http.StatusOK).
			JSON().Object().HasValue("email", "demo@example.com")
		findByID(t, exp.GET("/api/users").WithQuery("ids", ids).
			Expect().Status(http.StatusOK).JSON().Array(), restDemoUID.String()).
			HasValue("email", "demo@example.com")
	})
}

func findByID(t *testing.T, users *httpexpect.Array, id string) *httpexpect.Object {
	t.Helper()
	for _, v := range users.Iter() {
		if o := v.Object(); o.Value("id").String().Raw() == id {
			return o
		}
	}
	t.Fatalf("user %s not in response %v", id, users.Raw())
	return nil
}
