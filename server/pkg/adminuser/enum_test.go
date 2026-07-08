package adminuser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusFrom(t *testing.T) {
	tests := []struct {
		Name, Input string
		Expected    Status
		Err         error
	}{
		{Name: "pending", Input: "pending", Expected: StatusPending},
		{Name: "approved uppercase", Input: "APPROVED", Expected: StatusApproved},
		{Name: "rejected", Input: "rejected", Expected: StatusRejected},
		{Name: "invalid", Input: "xxx", Expected: Status("xxx"), Err: ErrInvalidStatus},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := StatusFrom(tt.Input)
			if tt.Err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.Expected, res)
			} else {
				assert.Equal(t, tt.Err, err)
			}
		})
	}
}

func TestStatus_Valid(t *testing.T) {
	assert.True(t, StatusPending.Valid())
	assert.True(t, StatusApproved.Valid())
	assert.True(t, StatusRejected.Valid())
	assert.False(t, Status("").Valid())
	assert.False(t, Status("unknown").Valid())
}

func TestStatus_String(t *testing.T) {
	assert.Equal(t, "pending", StatusPending.String())
}

func TestRoleFrom(t *testing.T) {
	tests := []struct {
		Name, Input string
		Expected    Role
		Err         error
	}{
		{Name: "system_admin", Input: "system_admin", Expected: RoleSystemAdmin},
		{Name: "viewer uppercase", Input: "VIEWER", Expected: RoleViewer},
		{Name: "invalid", Input: "xxx", Expected: Role("xxx"), Err: ErrInvalidRole},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := RoleFrom(tt.Input)
			if tt.Err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.Expected, res)
			} else {
				assert.Equal(t, tt.Err, err)
			}
		})
	}
}

func TestRole_Valid(t *testing.T) {
	assert.True(t, RoleSystemAdmin.Valid())
	assert.True(t, RoleViewer.Valid())
	assert.False(t, Role("").Valid())
	assert.False(t, Role("unknown").Valid())
}

func TestRole_String(t *testing.T) {
	assert.Equal(t, "system_admin", RoleSystemAdmin.String())
}
