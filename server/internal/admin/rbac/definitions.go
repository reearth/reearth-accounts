package rbac

import (
	"slices"

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
	ResourceUser      = "user"
	ResourceWorkspace = "workspace"
)

const (
	ActionDelete = "delete"
	ActionEdit   = "edit"
	ActionList   = "list"
	ActionRead   = "read"
)

// roleAdmin is the role granted to Re:Earth operational staff who access the
// internal management console. Admin operations are purely role-based.
const roleAdmin = "admin"

type ResourceRule struct {
	Resource string
	Actions  map[string][]string // action -> roles
}

var resourceRules = []ResourceRule{
	{
		Resource: ResourceUser,
		Actions: map[string][]string{
			ActionList:   {roleAdmin},
			ActionRead:   {roleAdmin},
			ActionEdit:   {roleAdmin},
			ActionDelete: {roleAdmin},
		},
	},
	{
		Resource: ResourceWorkspace,
		Actions: map[string][]string{
			ActionList:   {roleAdmin},
			ActionRead:   {roleAdmin},
			ActionEdit:   {roleAdmin},
			ActionDelete: {roleAdmin},
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
