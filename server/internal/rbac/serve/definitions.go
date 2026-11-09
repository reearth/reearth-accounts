package serve

import (
	"slices"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/samber/lo"
)

const (
	PolicyFileDir = "policies"
	ServiceName   = "serve"
)

const (
	ResourceProject   = "project"
	ResourceWorkspace = "workspace"
)

const (
	ActionCreateAsset    = "create_asset"
	ActionCreateProject  = "create_project"
	ActionDelete         = "delete"
	ActionDeleteAsset    = "delete_asset"
	ActionExtractAsset   = "extract_asset"
	ActionManageHosts    = "manage_hosts"
	ActionManageMembers  = "manage_members"
	ActionManageWebhooks = "manage_webhooks"
	ActionRead           = "read"
	ActionReadAsset      = "read_asset"
	ActionReadJob        = "read_job"
	ActionRetryJob       = "retry_job"
	ActionUpdateAsset    = "update_asset"
)

var ladder = []role.RoleType{role.RoleReader, role.RoleWriter, role.RoleMaintainer, role.RoleOwner}

// minimumRoles mirrors Serve's POLICY table (ADR-016 decision 10 in reearth/reearth-serve).
var minimumRoles = map[string]map[string]role.RoleType{
	ResourceProject: {
		ActionCreateAsset:    role.RoleWriter,
		ActionDelete:         role.RoleOwner,
		ActionDeleteAsset:    role.RoleWriter,
		ActionExtractAsset:   role.RoleWriter,
		ActionManageHosts:    role.RoleWriter,
		ActionManageWebhooks: role.RoleMaintainer,
		ActionRead:           role.RoleReader,
		ActionReadAsset:      role.RoleReader,
		ActionReadJob:        role.RoleReader,
		ActionRetryJob:       role.RoleWriter,
		ActionUpdateAsset:    role.RoleWriter,
	},
	ResourceWorkspace: {
		ActionCreateProject: role.RoleMaintainer,
		ActionDelete:        role.RoleOwner,
		ActionManageMembers: role.RoleMaintainer,
		ActionRead:          role.RoleReader,
	},
}

func DefineResources(builder *generator.ResourceBuilder) []generator.ResourceDefinition {
	if builder == nil {
		panic("ResourceBuilder cannot be nil")
	}

	for resource, actions := range minimumRoles {
		actionKeys := lo.Keys(actions)
		slices.Sort(actionKeys)

		defs := make([]generator.ActionDefinition, 0, len(actionKeys))
		for _, action := range actionKeys {
			defs = append(defs, generator.NewActionDefinition(action, rolesAtOrAbove(actions[action])))
		}
		builder.AddResource(resource, defs)
	}

	return builder.Build()
}

func rolesAtOrAbove(minimum role.RoleType) []string {
	i := slices.Index(ladder, minimum)
	if i < 0 {
		panic("role is not on the ladder: " + minimum.String())
	}
	return lo.Map(ladder[i:], func(r role.RoleType, _ int) string { return r.String() })
}
