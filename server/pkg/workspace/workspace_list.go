package workspace

import "github.com/reearth/reearth-accounts/server/pkg/role"

type List []*Workspace

func (l List) FilterByID(ids ...ID) List {
	if l == nil {
		return nil
	}

	byID := make(map[ID]*Workspace, len(l))
	for _, t := range l {
		byID[t.ID()] = t
	}

	res := make(List, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			res = append(res, t)
		}
	}
	return res
}

func (l List) FilterByUserRole(u UserID, r role.RoleType) List {
	if l == nil || u.IsEmpty() || r == "" {
		return nil
	}

	res := make(List, 0, len(l))
	for _, t := range l {
		if m := t.Members().User(u); m != nil && m.Role == r {
			res = append(res, t)
		}
	}
	return res
}

func (l List) FilterByIntegrationRole(i IntegrationID, r role.RoleType) List {
	if l == nil || i.IsEmpty() || r == "" {
		return nil
	}

	res := make(List, 0, len(l))
	for _, t := range l {
		if m := t.Members().Integration(i); m != nil && m.Role == r {
			res = append(res, t)
		}
	}
	return res
}

func (l List) FilterByUserRoleIncluding(u UserID, r role.RoleType) List {
	if l == nil || u.IsEmpty() || r == "" {
		return nil
	}

	res := make(List, 0, len(l))
	for _, t := range l {
		if m := t.Members().User(u); m != nil && m.Role.Includes(r) {
			res = append(res, t)
		}
	}
	return res
}

func (l List) FilterByIntegrationRoleIncluding(i IntegrationID, r role.RoleType) List {
	if l == nil || i.IsEmpty() || r == "" {
		return nil
	}

	res := make(List, 0, len(l))
	for _, t := range l {
		if m := t.Members().Integration(i); m != nil && m.Role.Includes(r) {
			res = append(res, t)
		}
	}
	return res
}

func (l List) IDs() []ID {
	if l == nil {
		return nil
	}

	res := make([]ID, 0, len(l))
	for _, t := range l {
		res = append(res, t.ID())
	}
	return res
}
