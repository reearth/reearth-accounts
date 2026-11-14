package serve

import (
	"os"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/rbac/policyrules"
	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func matrix() map[string]map[string][]string {
	got := map[string]map[string][]string{}
	for _, r := range DefineResources(generator.NewResourceBuilder(ServiceName)) {
		got[r.Resource] = map[string][]string{}
		for _, a := range r.Actions {
			got[r.Resource][a.Action] = a.Roles
		}
	}
	return got
}

func TestGeneratePolicies(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, generator.GeneratePolicies(ServiceName, policyrules.From(ServiceName, DefineResources), dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"serve_project.yaml", "serve_workspace.yaml"}, names)
}

// A change to Serve's POLICY table (core/auth/permissions.ts) must update this table.
func TestWantMatrix(t *testing.T) {
	assert.Equal(t, map[string]map[string][]string{
		"serve:project": {
			"create_asset":    []string{"writer", "maintainer", "owner"},
			"delete":          []string{"owner"},
			"delete_asset":    []string{"writer", "maintainer", "owner"},
			"extract_asset":   []string{"writer", "maintainer", "owner"},
			"manage_hosts":    []string{"writer", "maintainer", "owner"},
			"manage_webhooks": []string{"maintainer", "owner"},
			"read":            []string{"reader", "writer", "maintainer", "owner"},
			"read_asset":      []string{"reader", "writer", "maintainer", "owner"},
			"read_job":        []string{"reader", "writer", "maintainer", "owner"},
			"retry_job":       []string{"writer", "maintainer", "owner"},
			"update_asset":    []string{"writer", "maintainer", "owner"},
		},
		"serve:workspace": {
			"create_project": []string{"maintainer", "owner"},
			"delete":         []string{"owner"},
			"manage_members": []string{"maintainer", "owner"},
			"read":           []string{"reader", "writer", "maintainer", "owner"},
		},
	}, matrix())
}
