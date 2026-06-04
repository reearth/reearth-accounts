package rbac

import (
	"slices"

	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/samber/lo"
)

// ServiceName is the Cerbos service name for the admin bounded context. It is
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
// internal management console. Only this role is allowed to perform admin
// operations.
const roleAdmin = "admin"

type ResourceRule struct {
	Resource string
	Actions  map[string]ActionRule
}

type ActionRule struct {
	Roles     []string
	Condition *generator.Condition
}

// requireJWT gates every admin action behind a valid JWT, mirroring the
// regular accounts policies.
var requireJWT = &generator.Condition{
	Match: generator.Match{
		Expr: lo.ToPtr("has(request.auxData.jwt)"),
	},
}

var resourceRules = []ResourceRule{
	{
		Resource: ResourceUser,
		Actions: map[string]ActionRule{
			ActionList:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionRead:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionEdit:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionDelete: {Roles: []string{roleAdmin}, Condition: requireJWT},
		},
	},
	{
		Resource: ResourceWorkspace,
		Actions: map[string]ActionRule{
			ActionList:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionRead:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionEdit:   {Roles: []string{roleAdmin}, Condition: requireJWT},
			ActionDelete: {Roles: []string{roleAdmin}, Condition: requireJWT},
		},
	},
}

func DefineResources(builder *generator.ResourceBuilder) []generator.ResourceDefinition {
	if builder == nil {
		panic("ResourceBuilder cannot be nil")
	}

	for _, r := range resourceRules {
		var actions []generator.ActionDefinition
		// Sort action keys to ensure deterministic output
		actionKeys := make([]string, 0, len(r.Actions))
		for action := range r.Actions {
			actionKeys = append(actionKeys, action)
		}
		slices.Sort(actionKeys)

		for _, action := range actionKeys {
			actionRule := r.Actions[action]
			if actionRule.Condition != nil {
				actions = append(actions, generator.NewActionDefinitionWithCondition(action, actionRule.Roles, actionRule.Condition))
			} else {
				actions = append(actions, generator.NewActionDefinition(action, actionRule.Roles))
			}
		}
		builder.AddResource(r.Resource, actions)
	}

	return builder.Build()
}
