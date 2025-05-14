package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkspace_ID(t *testing.T) {
	wid := NewID()
	assert.Equal(t, wid, (&Workspace{id: wid}).ID())
}

func TestWorkspace_Name(t *testing.T) {
	assert.Equal(t, "x", (&Workspace{name: "x"}).Name())
}
func TestWorkspace_DisplayName(t *testing.T) {
	assert.Equal(t, "x", (&Workspace{displayName: "x"}).DisplayName())
}
func TestWorkspace_Members(t *testing.T) {
	m := NewMembersWith(map[UserID]Member{
		NewUserID(): {Role: RoleOwner},
	}, nil, false)
	assert.Equal(t, m, (&Workspace{members: m}).Members())
}

func TestWorkspace_IsPersonal(t *testing.T) {
	m := NewMembersWith(map[UserID]Member{
		NewUserID(): {Role: RoleOwner},
	}, nil, true)
	assert.True(t, (&Workspace{members: m}).IsPersonal())
	assert.False(t, (&Workspace{}).IsPersonal())
}

func TestWorkspace_Rename(t *testing.T) {
	w := &Workspace{}
	w.Rename("a")
	assert.Equal(t, "a", w.name)
}

func TestWorkspace_UpdateDisplayName(t *testing.T) {
	w := &Workspace{}
	w.UpdateDisplayName("a")
	assert.Equal(t, "a", w.displayName)
}

func TestWorkspace_Policy(t *testing.T) {
	w := &Workspace{}
	w.SetPolicy(PolicyID("ccc").Ref())
	assert.Equal(t, PolicyID("ccc").Ref(), w.Policy())
}
