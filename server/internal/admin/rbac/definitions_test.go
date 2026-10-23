package rbac

import (
	"testing"

	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/stretchr/testify/assert"
)

func actionRoles(t *testing.T, resource, action string) []string {
	t.Helper()
	for _, r := range DefineResources(generator.NewResourceBuilder(ServiceName)) {
		if r.Resource != ServiceName+":"+resource {
			continue
		}
		for _, a := range r.Actions {
			if a.Action == action {
				return a.Roles
			}
		}
	}
	t.Fatalf("action %s on %s is not defined", action, resource)
	return nil
}

func TestDefineResources_TransferOwnerIsSystemAdminOnly(t *testing.T) {
	assert.Equal(t, []string{roleSystemAdmin}, actionRoles(t, ResourceWorkspace, ActionTransferOwner))
}
