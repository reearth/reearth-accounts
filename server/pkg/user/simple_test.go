package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleFrom(t *testing.T) {
	u := &User{
		id:    NewID(),
		name:  "name",
		email: "email",
	}
	assert.Nil(t, SimpleFrom(nil))
	assert.Equal(t, &Simple{ID: u.id, Name: "name", Email: "email"}, SimpleFrom(u))
}
