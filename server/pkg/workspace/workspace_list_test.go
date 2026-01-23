package workspace

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/stretchr/testify/assert"
)

func TestWorkspaceList_FilterByID(t *testing.T) {
	tid1 := NewID()
	tid2 := NewID()
	t1 := &Workspace{id: tid1}
	t2 := &Workspace{id: tid2}

	assert.Equal(t, List{t1}, List{t1, t2}.FilterByID(tid1))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByID(tid2))
	assert.Equal(t, List{t1, t2}, List{t1, t2}.FilterByID(tid1, tid2))
	assert.Equal(t, List{}, List{t1, t2}.FilterByID(NewID()))
	assert.Equal(t, List(nil), List(nil).FilterByID(tid1))
}

func TestWorkspaceList_FilterByUserRole(t *testing.T) {
	uid := NewUserID()
	tid1 := NewID()
	tid2 := NewID()
	t1 := &Workspace{
		id: tid1,
		members: &Members{
			users: map[UserID]Member{
				uid: {Role: role.RoleReader},
			},
		},
	}
	t2 := &Workspace{
		id: tid2,
		members: &Members{
			users: map[UserID]Member{
				uid: {Role: role.RoleOwner},
			},
		},
	}

	assert.Equal(t, List{t1}, List{t1, t2}.FilterByUserRole(uid, role.RoleReader))
	assert.Equal(t, List{}, List{t1, t2}.FilterByUserRole(uid, role.RoleWriter))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByUserRole(uid, role.RoleOwner))
	assert.Equal(t, List(nil), List(nil).FilterByUserRole(uid, role.RoleOwner))
}

func TestWorkspaceList_FilterByIntegrationRole(t *testing.T) {
	iid := NewIntegrationID()
	tid1 := NewID()
	tid2 := NewID()
	t1 := &Workspace{
		id: tid1,
		members: &Members{
			integrations: map[IntegrationID]Member{
				iid: {Role: role.RoleReader},
			},
		},
	}
	t2 := &Workspace{
		id: tid2,
		members: &Members{
			integrations: map[IntegrationID]Member{
				iid: {Role: role.RoleWriter},
			},
		},
	}

	assert.Equal(t, List{t1}, List{t1, t2}.FilterByIntegrationRole(iid, role.RoleReader))
	assert.Equal(t, List{}, List{t1, t2}.FilterByIntegrationRole(iid, role.RoleOwner))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByIntegrationRole(iid, role.RoleWriter))
	assert.Equal(t, List(nil), List(nil).FilterByIntegrationRole(iid, role.RoleOwner))
}

func TestWorkspaceList_FilterByUserRoleIncluding(t *testing.T) {
	uid := NewUserID()
	tid1 := NewID()
	tid2 := NewID()
	t1 := &Workspace{
		id: tid1,
		members: &Members{
			users: map[UserID]Member{
				uid: {Role: role.RoleReader},
			},
		},
	}
	t2 := &Workspace{
		id: tid2,
		members: &Members{
			users: map[UserID]Member{
				uid: {Role: role.RoleOwner},
			},
		},
	}

	assert.Equal(t, List{t1, t2}, List{t1, t2}.FilterByUserRoleIncluding(uid, role.RoleReader))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByUserRoleIncluding(uid, role.RoleWriter))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByUserRoleIncluding(uid, role.RoleOwner))
	assert.Equal(t, List(nil), List(nil).FilterByUserRoleIncluding(uid, role.RoleOwner))
}

func TestWorkspaceList_FilterByIntegrationRoleIncluding(t *testing.T) {
	uid := NewIntegrationID()
	tid1 := NewID()
	tid2 := NewID()
	t1 := &Workspace{
		id: tid1,
		members: &Members{
			integrations: map[IntegrationID]Member{
				uid: {Role: role.RoleReader},
			},
		},
	}
	t2 := &Workspace{
		id: tid2,
		members: &Members{
			integrations: map[IntegrationID]Member{
				uid: {Role: role.RoleOwner},
			},
		},
	}

	assert.Equal(t, List{t1, t2}, List{t1, t2}.FilterByIntegrationRoleIncluding(uid, role.RoleReader))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByIntegrationRoleIncluding(uid, role.RoleWriter))
	assert.Equal(t, List{t2}, List{t1, t2}.FilterByIntegrationRoleIncluding(uid, role.RoleOwner))
	assert.Equal(t, List(nil), List(nil).FilterByIntegrationRoleIncluding(uid, role.RoleOwner))
}

func TestWorkspaceList_IDs(t *testing.T) {
	wid1 := NewID()
	wid2 := NewID()
	t1 := &Workspace{id: wid1}
	t2 := &Workspace{id: wid2}

	assert.Equal(t, []ID{wid1, wid2}, List{t1, t2}.IDs())
	assert.Equal(t, []ID{}, List{}.IDs())
	assert.Equal(t, []ID(nil), List(nil).IDs())
}
