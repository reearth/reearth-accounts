package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_TransferOwnership(t *testing.T) {
	ownerID := id.NewUserID()
	newOwnerID := id.NewUserID()
	id1 := id.NewWorkspaceID()

	// Initial state: Owner has Owner role, NewOwner has Maintainer role
	w1 := workspace.New().ID(id1).Name("W1").Members(map[user.ID]workspace.Member{
		ownerID:    {Role: role.RoleOwner},
		newOwnerID: {Role: role.RoleMaintainer},
	}).Personal(false).MustBuild()

	op := &workspace.Operator{
		User:               &ownerID,
		ReadableWorkspaces: []workspace.ID{id1},
		OwningWorkspaces:   []workspace.ID{id1},
	}

	ctx := context.Background()
	db := memory.New()
	// seed roles
	for _, r := range []string{"owner", "maintainer", "writer", "reader"} {
		_ = db.Role.Save(ctx, *role.New().NewID().Name(r).MustBuild())
	}

	_ = db.Workspace.Save(ctx, w1)

	workspaceUC := NewWorkspace(db, nil, nil)
	ws, err := workspaceUC.TransferOwnership(ctx, id1, newOwnerID, op)
	assert.NoError(t, err)
	assert.Equal(t, role.RoleOwner, ws.Members().UserRole(newOwnerID))
	assert.Equal(t, role.RoleMaintainer, ws.Members().UserRole(ownerID))

	// Verify Permittable for new owner
	pNew, err := db.Permittable.FindByUserID(ctx, newOwnerID)
	assert.NoError(t, err)
	assert.NotNil(t, pNew)

	found := false
	for _, wr := range pNew.WorkspaceRoles() {
		if wr.ID() == id1 {
			r, _ := db.Role.FindByID(ctx, wr.RoleID())
			assert.Equal(t, "owner", r.Name())
			found = true
			break
		}
	}
	assert.True(t, found, "New owner should have owner role in permittable")

	// Verify Permittable for old owner
	pOld, err := db.Permittable.FindByUserID(ctx, ownerID)
	assert.NoError(t, err)
	assert.NotNil(t, pOld)

	found = false
	for _, wr := range pOld.WorkspaceRoles() {
		if wr.ID() == id1 {
			r, _ := db.Role.FindByID(ctx, wr.RoleID())
			assert.Equal(t, "maintainer", r.Name())
			found = true
			break
		}
	}
	assert.True(t, found, "Old owner should have maintainer role in permittable")
}
