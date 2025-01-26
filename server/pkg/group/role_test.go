package group

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup_ID(t *testing.T) {
	var g *Group
	assert.Equal(t, ID{}, g.ID())

	expectedID := NewID()
	g = &Group{id: expectedID}
	assert.Equal(t, expectedID, g.ID())
}

func TestGroup_Name(t *testing.T) {
	var g *Group
	assert.Equal(t, "", g.Name())

	expectedName := "admin"
	g = &Group{name: expectedName}
	assert.Equal(t, expectedName, g.Name())
}

func TestGroup_Rename(t *testing.T) {
	var g *Group
	g.Rename("newName")
	assert.Nil(t, g)

	g = &Group{name: "admin"}
	newName := "member"
	g.Rename(newName)
	assert.Equal(t, newName, g.Name())
}
