package gqlmodel

import "github.com/reearth/reearth-accounts/server/pkg/role"

func ToRoles(roles role.List) []*RoleForAuthorization {
	res := make([]*RoleForAuthorization, 0, len(roles))
	for _, r := range roles {
		res = append(res, ToRoleForAuthorization(r))
	}
	return res
}

func ToRoleForAuthorization(r *role.Role) *RoleForAuthorization {
	return &RoleForAuthorization{
		ID:   IDFrom(r.ID()),
		Name: r.Name(),
	}
}
