package rbac

import (
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

const (
	roleReader     = "reader"
	roleWriter     = "writer"
	roleMaintainer = "maintainer"
	roleOwner      = "owner"
	roleSelf       = "self"
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

			// TODO: need further investigation about search user permission & validate alias permission
			ActionSearch:   {Roles: []string{roleReader, roleWriter, roleMaintainer, roleOwner}},
			ActionValidate: {Roles: []string{roleReader, roleWriter, roleMaintainer, roleOwner}},
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
		for action, actionRule := range r.Actions {
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
