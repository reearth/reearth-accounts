package e2e

import (
	"net/http"
	"testing"

	"github.com/eukarya-inc/reearth-accounts/internal/app"
	httpexpect "github.com/gavv/httpexpect/v2"
)

func checkPermission(e *httpexpect.Expect, service string, resource string, action string) (GraphQLRequest, *httpexpect.Value) {
	checkPermissionRequestBody := GraphQLRequest{
		OperationName: "CheckPermission",
		Query: `query CheckPermission($input: CheckPermissionInput!) {
			checkPermission(input: $input) {
				allowed
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"service":  service,
				"resource": resource,
				"action":   action,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(checkPermissionRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return checkPermissionRequestBody, res
}

func TestCheckPermission(t *testing.T) {
	// Start Cerbos container
	cerbosContainer, err := newCerbosContainer()
	if err != nil {
		t.Fatalf("failed to start cerbos container: %v", err)
	}
	defer func() {
		if err := cerbosContainer.terminate(); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}()

	// Start server
	e, _ := StartServer(t, &app.Config{
		CerbosHost: cerbosContainer.getAddress(),
	}, true, baseSeederOneUser)

	// check permission with no permittable
	_, res1 := checkPermission(e, "service", "resource", "read")
	res1.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsFalse()

	// Add role and permittable
	_, _, roleId1 := addRole(e, "role1")
	_, _, _ = updatePermittable(e, uId.String(), []string{roleId1})

	// check permission with permittable
	_, res2 := checkPermission(e, "service", "resource", "read")
	res2.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsTrue()

	// check permission with permittable but allowed is false
	_, res3 := checkPermission(e, "service", "resource", "edit")
	res3.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsFalse()
}
