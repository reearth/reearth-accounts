package gqlmodel

import "github.com/eukarya-inc/reearth-dashboard/pkg/group"

func ToGroups(groups group.List) []*Group {
	res := make([]*Group, 0, len(groups))
	for _, r := range groups {
		res = append(res, ToGroup(r))
	}
	return res
}

func ToGroup(r *group.Group) *Group {
	return &Group{
		ID:   IDFrom(r.ID()),
		Name: r.Name(),
	}
}
