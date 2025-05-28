package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoleFrom(t *testing.T) {
	tests := []struct {
		Name, Role string
		Expected   Role
		Err        error
	}{
		{
			Name:     "Success reader",
			Role:     "reader",
			Expected: RoleReader,
			Err:      nil,
		},
		{
			Name:     "fail invalid role",
			Role:     "xxx",
			Expected: Role("xxx"),
			Err:      ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := RoleFrom(tt.Role)
			if tt.Err == nil {
				assert.Equal(t, tt.Expected, res)
			} else {
				assert.Equal(t, tt.Err, err)
			}
		})
	}
}

func TestRole_Valid(t *testing.T) {
	assert.True(t, RoleOwner.Valid())
	assert.True(t, RoleMaintainer.Valid())
	assert.True(t, RoleWriter.Valid())
	assert.True(t, RoleReader.Valid())
	assert.False(t, Role("").Valid())
}

func TestRole_Includes(t *testing.T) {
	assert.True(t, RoleOwner.Includes(RoleOwner))
	assert.True(t, RoleOwner.Includes(RoleMaintainer))
	assert.True(t, RoleOwner.Includes(RoleWriter))
	assert.True(t, RoleOwner.Includes(RoleReader))

	assert.False(t, RoleMaintainer.Includes(RoleOwner))
	assert.True(t, RoleMaintainer.Includes(RoleMaintainer))
	assert.True(t, RoleMaintainer.Includes(RoleWriter))
	assert.True(t, RoleMaintainer.Includes(RoleReader))

	assert.False(t, RoleWriter.Includes(RoleOwner))
	assert.False(t, RoleWriter.Includes(RoleMaintainer))
	assert.True(t, RoleWriter.Includes(RoleWriter))
	assert.True(t, RoleWriter.Includes(RoleReader))

	assert.False(t, RoleReader.Includes(RoleOwner))
	assert.False(t, RoleReader.Includes(RoleMaintainer))
	assert.False(t, RoleReader.Includes(RoleWriter))
	assert.True(t, RoleReader.Includes(RoleReader))

	assert.False(t, Role("").Includes(RoleReader))
}
