package permittable

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
)

func TestRole_ID(t *testing.T) {
	var p *Permittable
	assert.Equal(t, ID{}, p.ID())

	expectedID := NewID()
	p = &Permittable{id: expectedID}
	assert.Equal(t, expectedID, p.ID())
}

func TestRole_UserID(t *testing.T) {
	var p *Permittable
	assert.Equal(t, user.ID{}, p.UserID())

	expectedUserID := user.NewID()
	p = &Permittable{userID: expectedUserID}
	assert.Equal(t, expectedUserID, p.UserID())
}

func TestRole_RoleIDs(t *testing.T) {
	var p *Permittable
	assert.Nil(t, p.RoleIDs())

	expectedRoleIDs := []id.RoleID{id.NewRoleID(), id.NewRoleID()}
	p = &Permittable{roleIDs: expectedRoleIDs}
	assert.Equal(t, expectedRoleIDs, p.RoleIDs())
}

func TestRole_EditRoleIDs(t *testing.T) {
	var p *Permittable
	p.EditRoleIDs(nil)
	assert.Nil(t, p)

	p = &Permittable{}
	oldRoleIDs := []id.RoleID{id.NewRoleID(), id.NewRoleID()}
	p = &Permittable{roleIDs: oldRoleIDs}

	newRoleIDs := []id.RoleID{id.NewRoleID(), id.NewRoleID()}
	p.EditRoleIDs(newRoleIDs)
	assert.Equal(t, newRoleIDs, p.RoleIDs())
}
