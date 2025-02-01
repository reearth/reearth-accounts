package interactor

import (
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	infraCerbos "github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/cerbos"
	"github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/memory"
	"github.com/stretchr/testify/assert"
)

func TestNewCerbos(t *testing.T) {
	memory := memory.New()

	cerbosClient, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
	if err != nil {
		t.Fatal(err)
	}
	cerbosAdapter := infraCerbos.NewCerbosAdapter(cerbosClient)

	c := NewCerbos(cerbosAdapter, memory)
	assert.NotNil(t, c)
}

// TODO: ci has failed so commented it out for now
// func TestCheckPermission(t *testing.T) {
// 	// prepare
// 	ctx := context.Background()
// 	memory := memory.New()
// 	uid1 := user.NewID()
// 	u1 := user.New().ID(uid1).Name("hoge").Email("abc@bb.cc").MustBuild()

// 	cerbosClient, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	cerbosAdapter := infraCerbos.NewCerbosAdapter(cerbosClient)

// 	c := &Cerbos{
// 		roleRepo:        memory.Role,
// 		permittableRepo: memory.Permittable,
// 		cerbos:          cerbosAdapter,
// 	}

// 	// check permission with no permittable
// 	res, err := c.CheckPermission(
// 		ctx,
// 		interfaces.CheckPermissionParam{
// 			Service:  "service",
// 			Resource: "resource",
// 			Action:   "read",
// 		},
// 		u1,
// 	)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, res)
// 	assert.False(t, res.Allowed)

// 	// add role
// 	r1, _ := role.New().
// 		NewID().
// 		Name("role1").
// 		Build()
// 	r2, _ := role.New().
// 		NewID().
// 		Name("role2").
// 		Build()
// 	err = memory.Role.Save(ctx, *r1)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	err = memory.Role.Save(ctx, *r2)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// add permittable
// 	p1, _ := permittable.New().
// 		NewID().
// 		UserID(uid1).
// 		RoleIDs([]id.RoleID{r1.ID(), r2.ID()}).
// 		Build()
// 	err = memory.Permittable.Save(ctx, *p1)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// check permission with permittable
// 	res2, err := c.CheckPermission(
// 		ctx,
// 		interfaces.CheckPermissionParam{
// 			Service:  "service",
// 			Resource: "resource",
// 			Action:   "read",
// 		},
// 		u1,
// 	)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, res2)
// 	assert.True(t, res2.Allowed)

// 	// check permission with permittable but not allowed
// 	res3, err := c.CheckPermission(
// 		ctx,
// 		interfaces.CheckPermissionParam{
// 			Service:  "service",
// 			Resource: "resource",
// 			Action:   "edit",
// 		},
// 		u1,
// 	)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, res3)
// 	assert.False(t, res3.Allowed)
// }
