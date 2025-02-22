package interactor

import (
	"context"
	"testing"

	"github.com/eukarya-inc/reearth-accounts/internal/infrastructure/memory"
	"github.com/eukarya-inc/reearth-accounts/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-accounts/pkg/id"
	"github.com/eukarya-inc/reearth-accounts/pkg/permittable"
	"github.com/eukarya-inc/reearth-accounts/pkg/role"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/stretchr/testify/assert"
)

func TestGetRoles(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	r := &Role{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// get roles successfully with no roles
	roles, err := r.GetRoles(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, roles)
	assert.Equal(t, role.List{}, roles)

	// add 2 roles
	r1, _ := role.New().
		NewID().
		Name("hoge").
		Build()
	r2, _ := role.New().
		NewID().
		Name("foo").
		Build()
	err = memory.Role.Save(ctx, *r1)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Role.Save(ctx, *r2)
	if err != nil {
		t.Fatal(err)
	}

	// get roles successfully with 2 roles
	roles, err = r.GetRoles(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, roles)
	assert.Equal(t, 2, len(roles))
}

func TestAddRole(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	r := &Role{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add role successfully
	role, err := r.AddRole(ctx, interfaces.AddRoleParam{
		Name: "test",
	})
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "test", role.Name())

	// add role with empty name should fail
	role, err = r.AddRole(ctx, interfaces.AddRoleParam{
		Name: "",
	})
	assert.Error(t, err)
	assert.Nil(t, role)
}

func TestUpdateRole(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	r := &Role{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add role
	r1, _ := role.New().
		NewID().
		Name("hoge").
		Build()
	err := memory.Role.Save(ctx, *r1)
	if err != nil {
		t.Fatal(err)
	}

	// update role successfully
	updatedRole, err := r.UpdateRole(ctx, interfaces.UpdateRoleParam{
		ID:   r1.ID(),
		Name: "foo",
	})
	assert.NoError(t, err)
	assert.NotNil(t, updatedRole)
	assert.Equal(t, r1.ID(), updatedRole.ID())
	assert.Equal(t, "foo", updatedRole.Name())

	// update role with non-existing role should fail
	updatedRole, err = r.UpdateRole(ctx, interfaces.UpdateRoleParam{
		ID:   role.NewID(),
		Name: "bar",
	})
	assert.Error(t, err)
	assert.Nil(t, updatedRole)
}

func TestRemoveRole(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	r := &Role{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add role
	rid := role.NewID()
	r1, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()
	err := memory.Role.Save(ctx, *r1)
	if err != nil {
		t.Fatal(err)
	}

	// remove role successfully
	roleID, err := r.RemoveRole(ctx, interfaces.RemoveRoleParam{
		ID: r1.ID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, r1.ID(), roleID)

	// remove role with non-existing role should fail
	roleID, err = r.RemoveRole(ctx, interfaces.RemoveRoleParam{
		ID: rid,
	})
	assert.Error(t, err)
	assert.Equal(t, role.ID{}, roleID)

	// remove role should failed because the role is used by a permittable
	rid2 := role.NewID()
	r2, _ := role.New().
		ID(rid2).
		Name("foo").
		Build()
	err = memory.Role.Save(ctx, *r2)
	if err != nil {
		t.Fatal(err)
	}

	uid := user.NewID()
	p, _ := permittable.New().
		NewID().
		UserID(uid).
		RoleIDs([]id.RoleID{rid2}).
		Build()
	err = memory.Permittable.Save(ctx, *p)
	if err != nil {
		t.Fatal(err)
	}

	roleID, err = r.RemoveRole(ctx, interfaces.RemoveRoleParam{
		ID: rid2,
	})
	assert.Error(t, err)
	assert.Equal(t, role.ID{}, roleID)
}
