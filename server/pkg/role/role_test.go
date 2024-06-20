package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRole_ID(t *testing.T) {
	var r *Role
	assert.Equal(t, ID{}, r.ID())

	expectedID := NewID()
	r = &Role{id: expectedID}
	assert.Equal(t, expectedID, r.ID())
}

func TestRole_Name(t *testing.T) {
	var r *Role
	assert.Equal(t, "", r.Name())

	expectedName := "admin"
	r = &Role{name: expectedName}
	assert.Equal(t, expectedName, r.Name())
}

func TestRole_Rename(t *testing.T) {
	var r *Role
	r.Rename("newName")
	assert.Nil(t, r)

	r = &Role{name: "admin"}
	newName := "member"
	r.Rename(newName)
	assert.Equal(t, newName, r.Name())
}
