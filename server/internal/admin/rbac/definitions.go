package rbac

import (
	"slices"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/cerbos/generator"
)

// ServiceName is the Cerbos service name for the admin bounded context,
// intentionally distinct from the regular "accounts" service so admin
// authorization is evaluated against a separate policy set.
const (
	ServiceName   = "accounts-admin"
	PolicyFileDir = "policies"
)

const (
	ResourceAdminUser = "admin_user"
	ResourceUser      = "user"
	ResourceWorkspace = "workspace"
)

const (
	ActionApprove    = "approve"
	ActionAssignRole = "assign_role"
	ActionDelete     = "delete"
	ActionEdit       = "edit"
	ActionList       = "list"
	ActionRead       = "read"
	ActionReadMember = "read_member"
	ActionReject     = "reject"
)

// roleSystemAdmin and roleViewer are the admin console roles. They reference the
// domain enum in pkg/adminuser so the Cerbos principal roles and the matrix
// roles cannot drift.
var (
	roleSystemAdmin = adminuser.RoleSystemAdmin.String()
	roleViewer      = adminuser.RoleViewer.String()
)

type ResourceRule struct {
	Resource string
	Actions  map[string][]string // action -> roles
}

var resourceRules = []ResourceRule{
	{
		Resource: ResourceAdminUser,
		Actions: map[string][]string{
			ActionList:       {roleSystemAdmin, roleViewer},
			ActionApprove:    {roleSystemAdmin},
			ActionReject:     {roleSystemAdmin},
			ActionAssignRole: {roleSystemAdmin},
		},
	},
	{
		Resource: ResourceUser,
		Actions: map[string][]string{
			ActionList:   {roleSystemAdmin, roleViewer},
			ActionRead:   {roleSystemAdmin, roleViewer},
			ActionEdit:   {roleSystemAdmin},
			ActionDelete: {roleSystemAdmin},
		},
	},
	{
		Resource: ResourceWorkspace,
		Actions: map[string][]string{
			ActionList:       {roleSystemAdmin, roleViewer},
			ActionRead:       {roleSystemAdmin, roleViewer},
			ActionReadMember: {roleSystemAdmin, roleViewer},
			ActionEdit:       {roleSystemAdmin},
			ActionDelete:     {roleSystemAdmin},
		},
	},
}

func DefineResources(builder *generator.ResourceBuilder) []generator.ResourceDefinition {
	if builder == nil {
		panic("ResourceBuilder cannot be nil")
	}

	for _, r := range resourceRules {
		actionKeys := make([]string, 0, len(r.Actions))
		for action := range r.Actions {
			actionKeys = append(actionKeys, action)
		}
		slices.Sort(actionKeys)

		actions := make([]generator.ActionDefinition, 0, len(actionKeys))
		for _, action := range actionKeys {
			actions = append(actions, generator.NewActionDefinition(action, r.Actions[action]))
		}
		builder.AddResource(r.Resource, actions)
	}

	return builder.Build()
}
