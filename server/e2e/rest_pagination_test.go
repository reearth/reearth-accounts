package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
)

func TestREST_Pagination(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	ids := restDemoUID.String() + "," + restOtherUID.String()

	// Users paginated form returns the PageResult shape.
	obj := exp.GET("/api/users").
		WithQuery("ids", ids).
		WithQuery("page", "1").
		WithQuery("page_size", "2").
		Expect().Status(http.StatusOK).JSON().Object()
	obj.Value("items").Array().Length().IsEqual(2)
	pg := obj.Value("pagination").Object()
	pg.Value("page").Number().IsEqual(1)
	pg.Value("page_size").Number().IsEqual(2)
	pg.Value("total").Number().IsEqual(2)

	// page_size above the max is clamped to 100 (reflected in the response metadata).
	obj2 := exp.GET("/api/users").
		WithQuery("ids", ids).
		WithQuery("page", "1").
		WithQuery("page_size", "999").
		Expect().Status(http.StatusOK).JSON().Object()
	obj2.Value("pagination").Object().Value("page_size").Number().IsEqual(100)

	// Workspaces paginated-by-user form returns the same PageResult shape.
	wobj := exp.GET("/api/workspaces").
		WithQuery("user_id", restDemoUID.String()).
		WithQuery("page", "1").
		WithQuery("page_size", "2").
		Expect().Status(http.StatusOK).JSON().Object()
	wobj.ContainsKey("items")
	wpg := wobj.Value("pagination").Object()
	wpg.Value("page").Number().IsEqual(1)
	wpg.Value("page_size").Number().IsEqual(2)
}

// --- Mock_Auth=false (real JWT pipeline) variant ---

const realJWTPaginationSub = "test|realjwt-pagination"

func TestREST_RealJWT_Pagination(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUsers(realJWTPaginationSub))
	token := signTestToken(t, key, realJWTPaginationSub)
	bearer := "Bearer " + token

	ids := jwtPrimaryUID.String() + "," + jwtSecondUID.String()

	// Users paginated form -> PageResult shape.
	obj := exp.GET("/api/users").
		WithQuery("ids", ids).
		WithQuery("page", "1").
		WithQuery("page_size", "2").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).JSON().Object()
	obj.Value("items").Array().Length().IsEqual(2)
	pg := obj.Value("pagination").Object()
	pg.Value("page").Number().IsEqual(1)
	pg.Value("page_size").Number().IsEqual(2)
	pg.Value("total").Number().IsEqual(2)

	// page_size > max is clamped to 100.
	obj2 := exp.GET("/api/users").
		WithQuery("ids", ids).
		WithQuery("page", "1").
		WithQuery("page_size", "999").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).JSON().Object()
	obj2.Value("pagination").Object().Value("page_size").Number().IsEqual(100)

	// Workspaces paginated by user_id.
	wobj := exp.GET("/api/workspaces").
		WithQuery("user_id", jwtPrimaryUID.String()).
		WithQuery("page", "1").
		WithQuery("page_size", "2").
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).JSON().Object()
	wobj.ContainsKey("items")
	wpg := wobj.Value("pagination").Object()
	wpg.Value("page").Number().IsEqual(1)
	wpg.Value("page_size").Number().IsEqual(2)
}
