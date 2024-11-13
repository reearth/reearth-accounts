package gqlmodel

import (
	"github.com/eukarya-inc/reearth-dashboard/pkg/permittable"
	"github.com/eukarya-inc/reearth-dashboard/pkg/role"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

func ToUsersWithRoles(users user.List, userRoleMap map[user.ID]role.List) []*UserWithRoles {
	result := make([]*UserWithRoles, 0, len(users))
	for _, user := range users {
		roles := make([]*RoleForAuthorization, 0)
		userRoles, ok := userRoleMap[user.ID()]
		if ok {
			for _, r := range userRoles {
				roles = append(roles, ToRoleForAuthorization(r))
			}
		}

		userWithRoles := &UserWithRoles{
			User:  ToUserForAuthorization(user),
			Roles: roles,
		}
		result = append(result, userWithRoles)
	}
	return result
}

func ToPermittable(u *permittable.Permittable) *Permittable {
	roleIds := make([]ID, 0, len(u.RoleIDs()))
	for _, rid := range u.RoleIDs() {
		roleIds = append(roleIds, IDFrom(rid))
	}

	return &Permittable{
		ID:      IDFrom(u.ID()),
		UserID:  IDFrom(u.UserID()),
		RoleIds: roleIds,
	}
}
