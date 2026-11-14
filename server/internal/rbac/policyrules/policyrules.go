// Package policyrules adapts builder-style resource definitions to the
// []generator.ResourceRule input that reearthx's policy generator takes.
package policyrules

import (
	"strings"

	"github.com/reearth/reearthx/cerbos/generator"
)

func From(serviceName string, define generator.DefineResourcesFunc) []generator.ResourceRule {
	defs := define(generator.NewResourceBuilder(serviceName))
	rules := make([]generator.ResourceRule, 0, len(defs))
	for _, d := range defs {
		actions := make(map[string]generator.ActionRule, len(d.Actions))
		for _, a := range d.Actions {
			actions[a.Action] = generator.ActionRule{Roles: a.Roles, Condition: a.Condition}
		}
		rules = append(rules, generator.ResourceRule{
			Resource: strings.TrimPrefix(d.Resource, serviceName+":"),
			Actions:  actions,
		})
	}
	return rules
}
