package rbac

import (
	"slices"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/samber/lo"
)

const (
	ServiceName   = "accounts"
	PolicyFileDir = "policies"
)

const (
	ResourceUser      = "user"
	ResourceWorkspace = "workspace"
)

const (
	ActionAddMember         = "add_member"
	ActionCreate            = "create"
	ActionDelete            = "delete"
	ActionDeleteMember      = "delete_member"
	ActionEdit              = "edit"
	ActionEditAlias         = "edit_alias"
	ActionEditMember        = "edit_member"
	ActionList              = "list"
	ActionRead              = "read"
	ActionReadMember        = "read_member"
	ActionSearch            = "search"
	ActionTransferOwnership = "transfer_ownership"
	ActionValidate          = "validate"
)

var (
	roleReader     = role.RoleReader.String()
	roleWriter     = role.RoleWriter.String()
	roleMaintainer = role.RoleMaintainer.String()
	roleOwner      = role.RoleOwner.String()
	roleSelf       = role.RoleSelf.String()
)

type ResourceRule struct {
	Resource string
	Actions  map[string]ActionRule
}

type ActionRule struct {
	Roles     []string
	Condition *generator.Condition
}

var resourceRules = []ResourceRule{
	{
		Resource: ResourceUser,
		Actions: map[string]ActionRule{
			ActionDelete: {
				Roles: []string{roleSelf},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},
			ActionEdit: {
				Roles: []string{roleSelf},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},
			ActionRead: {
				Roles: []string{roleSelf},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},

			ActionSearch: {
				Roles: []string{roleSelf, roleReader, roleWriter, roleMaintainer, roleOwner},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},
			ActionValidate: {
				Roles: []string{roleSelf, roleReader, roleWriter, roleMaintainer, roleOwner},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},
		},
	},
	{
		Resource: ResourceWorkspace,
		Actions: map[string]ActionRule{
			ActionCreate: {
				Roles: []string{roleSelf},
				Condition: &generator.Condition{
					Match: generator.Match{
						Expr: lo.ToPtr("has(request.auxData.jwt)"),
					},
				},
			},
			ActionDelete:            {Roles: []string{roleOwner}},
			ActionEdit:              {Roles: []string{roleMaintainer, roleOwner}},
			ActionEditAlias:         {Roles: []string{roleOwner}},
			ActionList:              {Roles: []string{roleSelf, roleReader, roleWriter, roleMaintainer, roleOwner}},
			ActionRead:              {Roles: []string{roleReader, roleWriter, roleMaintainer, roleOwner}},
			ActionTransferOwnership: {Roles: []string{roleOwner}},
			ActionValidate:          {Roles: []string{roleReader, roleWriter, roleMaintainer, roleOwner}},

			// Members
			ActionAddMember:    {Roles: []string{roleMaintainer, roleOwner}},
			ActionEditMember:   {Roles: []string{roleMaintainer, roleOwner}},
			ActionDeleteMember: {Roles: []string{roleMaintainer, roleOwner}},
			ActionReadMember:   {Roles: []string{roleReader, roleWriter, roleMaintainer, roleOwner}},
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
