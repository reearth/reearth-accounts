package interactor

import (
	"context"
	"testing"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersWithRoles(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := accountmemory.New()
	uid1 := user.NewID()
	uid2 := user.NewID()
	u1 := user.New().ID(uid1).Name("hoge").Email("abc@bb.cc").MustBuild()
	u2 := user.New().ID(uid2).Name("foo").Email("cba@bb.cc").MustBuild()
	userRepo := repo.NewMultiUser(
		accountmemory.NewUserWith(u1, u2),
	)
	p := &Permittable{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		userRepo:        userRepo,
		transaction:     memory.Transaction,
	}

	rid1 := role.NewID()
	rid2 := role.NewID()
	r1 := role.New().ID(rid1).Name("hoge").MustBuild()
	r2 := role.New().ID(rid2).Name("foo").MustBuild()
	err := memory.Role.Save(ctx, *r1)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Role.Save(ctx, *r2)
	if err != nil {
		t.Fatal(err)
	}

	p1, _ := permittable.New().
		NewID().
		UserID(uid1).
		RoleIDs([]id.RoleID{rid1}).
		Build()
	p2, _ := permittable.New().
		NewID().
		UserID(uid2).
		RoleIDs([]id.RoleID{rid1, rid2}).
		Build()
	err = memory.Permittable.Save(ctx, *p1)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Permittable.Save(ctx, *p2)
	if err != nil {
		t.Fatal(err)
	}

	// get users with roles successfully
	users, userIdRoleMap, err := p.GetUsersWithRoles(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, users)
	assert.Equal(t, 2, len(users))

	assert.NotNil(t, userIdRoleMap)
	assert.Equal(t, 2, len(userIdRoleMap))
	assert.Equal(t, role.List{r1}, userIdRoleMap[uid1])
	assert.Equal(t, role.List{r1, r2}, userIdRoleMap[uid2])
}

func TestUpdatePermittable(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := accountmemory.New()
	uid1 := user.NewID()
	u1 := user.New().ID(uid1).Name("hoge").Email("abc@bb.cc").MustBuild()
	userRepo := repo.NewMultiUser(
		accountmemory.NewUserWith(u1),
	)
	p := &Permittable{
		roleRepo:        memory.Role,
		permittableRepo: memory.Permittable,
		userRepo:        userRepo,
		transaction:     memory.Transaction,
	}

	rid1 := role.NewID()
	rid2 := role.NewID()
	rid3 := role.NewID()
	r1 := role.New().ID(rid1).Name("hoge").MustBuild()
	r2 := role.New().ID(rid2).Name("foo").MustBuild()
	r3 := role.New().ID(rid3).Name("bar").MustBuild()
	err := memory.Role.Save(ctx, *r1)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Role.Save(ctx, *r2)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Role.Save(ctx, *r3)
	if err != nil {
		t.Fatal(err)
	}

	p1, _ := permittable.New().
		NewID().
		UserID(uid1).
		RoleIDs([]id.RoleID{rid1, rid2}).
		Build()
	err = memory.Permittable.Save(ctx, *p1)
	if err != nil {
		t.Fatal(err)
	}

	// update permittable successfully
	updatedPermittable, err := p.UpdatePermittable(ctx, interfaces.UpdatePermittableParam{
		UserID:  uid1,
		RoleIDs: []id.RoleID{rid1, rid3},
	})
	assert.NoError(t, err)
	assert.Equal(t, uid1, updatedPermittable.UserID())
	assert.Equal(t, []id.RoleID{rid1, rid3}, updatedPermittable.RoleIDs())

	// add new permittable successfully
	uid2 := user.NewID()
	updatedPermittable, err = p.UpdatePermittable(ctx, interfaces.UpdatePermittableParam{
		UserID:  uid2,
		RoleIDs: []id.RoleID{rid1, rid2, rid3},
	})
	assert.NoError(t, err)
	assert.Equal(t, uid2, updatedPermittable.UserID())
	assert.Equal(t, []id.RoleID{rid1, rid2, rid3}, updatedPermittable.RoleIDs())
}
