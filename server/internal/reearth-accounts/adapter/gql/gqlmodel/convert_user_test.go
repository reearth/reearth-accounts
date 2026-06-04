package gqlmodel

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/stretchr/testify/assert"
)

func TestToRole(t *testing.T) {
	tests := []struct {
		name string
		arg  role.RoleType
		want Role
	}{
		{
			name: "RoleOwner",
			arg:  role.RoleOwner,
			want: RoleOwner,
		},
		{
			name: "RoleMaintainer",
			arg:  role.RoleMaintainer,
			want: RoleMaintainer,
		},
		{
			name: "RoleWriter",
			arg:  role.RoleWriter,
			want: RoleWriter,
		},
		{
			name: "RoleReader",
			arg:  role.RoleReader,
			want: RoleReader,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ToRole(tt.arg))
		})
	}
}

func TestFromRole(t *testing.T) {
	tests := []struct {
		name string
		arg  Role
		want role.RoleType
	}{
		{
			name: "RoleOwner",
			arg:  RoleOwner,
			want: role.RoleOwner,
		},
		{
			name: "RoleMaintainer",
			arg:  RoleMaintainer,
			want: role.RoleMaintainer,
		},
		{
			name: "RoleWriter",
			arg:  RoleWriter,
			want: role.RoleWriter,
		},
		{
			name: "RoleReader",
			arg:  RoleReader,
			want: role.RoleReader,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, FromRole(tt.arg))
		})
	}
}
