package interactor

import (
	"context"
	"testing"

	"github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/memory"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/eukarya-inc/reearth-dashboard/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/stretchr/testify/assert"
)

func TestGetGroups(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	g := &Group{
		groupRepo:       memory.Group,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// get groups successfully with no groups
	groups, err := g.GetGroups(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, groups)
	assert.Equal(t, group.List{}, groups)

	// add 2 groups
	g1, _ := group.New().
		NewID().
		Name("hoge").
		Build()
	g2, _ := group.New().
		NewID().
		Name("foo").
		Build()
	err = memory.Group.Save(ctx, *g1)
	if err != nil {
		t.Fatal(err)
	}
	err = memory.Group.Save(ctx, *g2)
	if err != nil {
		t.Fatal(err)
	}

	// get groups successfully with 2 groups
	groups, err = g.GetGroups(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, groups)
	assert.Equal(t, 2, len(groups))
}

func TestAddGroup(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	g := &Group{
		groupRepo:       memory.Group,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add group successfully
	group, err := g.AddGroup(ctx, interfaces.AddGroupParam{
		Name: "test",
	})
	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, "test", group.Name())

	// add group with empty name should fail
	group, err = g.AddGroup(ctx, interfaces.AddGroupParam{
		Name: "",
	})
	assert.Error(t, err)
	assert.Nil(t, group)
}

func TestUpdateGroup(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	g := &Group{
		groupRepo:       memory.Group,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add group
	g1, _ := group.New().
		NewID().
		Name("hoge").
		Build()
	err := memory.Group.Save(ctx, *g1)
	if err != nil {
		t.Fatal(err)
	}

	// update group successfully
	updatedGroup, err := g.UpdateGroup(ctx, interfaces.UpdateGroupParam{
		ID:   g1.ID(),
		Name: "foo",
	})
	assert.NoError(t, err)
	assert.NotNil(t, updatedGroup)
	assert.Equal(t, g1.ID(), updatedGroup.ID())
	assert.Equal(t, "foo", updatedGroup.Name())

	// update group with non-existing group should fail
	updatedGroup, err = g.UpdateGroup(ctx, interfaces.UpdateGroupParam{
		ID:   group.NewID(),
		Name: "bar",
	})
	assert.Error(t, err)
	assert.Nil(t, updatedGroup)
}

func TestRemoveGroup(t *testing.T) {
	// prepare
	ctx := context.Background()
	memory := memory.New()
	g := &Group{
		groupRepo:       memory.Group,
		permittableRepo: memory.Permittable,
		transaction:     memory.Transaction,
	}

	// add group
	gid := group.NewID()
	g1, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()
	err := memory.Group.Save(ctx, *g1)
	if err != nil {
		t.Fatal(err)
	}

	// remove group successfully
	groupID, err := g.RemoveGroup(ctx, interfaces.RemoveGroupParam{
		ID: g1.ID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, g1.ID(), groupID)

	// remove group with non-existing group should fail
	groupID, err = g.RemoveGroup(ctx, interfaces.RemoveGroupParam{
		ID: gid,
	})
	assert.Error(t, err)
	assert.Equal(t, group.ID{}, groupID)

	// remove group should failed because the group is used by a permittable
	gid2 := group.NewID()
	g2, _ := group.New().
		ID(gid2).
		Name("foo").
		Build()
	err = memory.Group.Save(ctx, *g2)
	if err != nil {
		t.Fatal(err)
	}

	uid := user.NewID()
	p, _ := permittable.New().
		NewID().
		UserID(uid).
		GroupIDs([]id.GroupID{gid2}).
		Build()
	err = memory.Permittable.Save(ctx, *p)
	if err != nil {
		t.Fatal(err)
	}

	groupID, err = g.RemoveGroup(ctx, interfaces.RemoveGroupParam{
		ID: gid2,
	})
	assert.Error(t, err)
	assert.Equal(t, group.ID{}, groupID)
}
