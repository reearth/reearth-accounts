package migration

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

/*
Fix permittable.workspace_roles to match the current workspace membership.
This migration rebuilds workspace_roles for each permittable based on:
1. The user must be a member of the workspace
2. The role must match the current role in workspace.members
*/

func FixPermittableWorkspaceRoles(ctx context.Context, c DBClient) error {
	roleCol := c.Collection("role")
	permittableCol := c.Collection("permittable")
	workspaceCol := c.Collection("workspace")

	// Step 1: Load all roles and create maps for role name <-> role ID
	roleNameToID := make(map[string]string)
	roleIDToName := make(map[string]string)
	err := roleCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var roleDoc mongodoc.RoleDocument
				if err := bson.Unmarshal(row, &roleDoc); err != nil {
					return err
				}
				roleNameToID[roleDoc.Name] = roleDoc.ID
				roleIDToName[roleDoc.ID] = roleDoc.Name
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Step 2: Build expected workspace roles for each user from workspace memberships
	// Map: userID -> []WorkspaceRoleDocument
	expectedWorkspaceRoles := make(map[string][]mongodoc.WorkspaceRoleDocument)
	err = workspaceCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var wsDoc mongodoc.WorkspaceDocument
				if err := bson.Unmarshal(row, &wsDoc); err != nil {
					return err
				}

				// Process each member in the workspace
				for userIDStr, member := range wsDoc.Members {
					// Get the role ID from the role name
					roleID, exists := roleNameToID[member.Role]
					if !exists {
						// Skip if role doesn't exist
						continue
					}

					workspaceRole := mongodoc.WorkspaceRoleDocument{
						WorkspaceID: wsDoc.ID,
						RoleID:      roleID,
					}

					expectedWorkspaceRoles[userIDStr] = append(expectedWorkspaceRoles[userIDStr], workspaceRole)
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Step 3: Update all permittables with correct workspace_roles
	return permittableCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var permittableDoc mongodoc.PermittableDocument
				if err := bson.Unmarshal(row, &permittableDoc); err != nil {
					return err
				}

				// Get expected workspace roles for this user
				expected := expectedWorkspaceRoles[permittableDoc.UserID]

				// Check if update is needed
				if workspaceRolesEqual(permittableDoc.WorkspaceRoles, expected) {
					continue
				}

				// Update workspace_roles
				permittableDoc.WorkspaceRoles = expected
				if err := permittableCol.SaveOne(ctx, permittableDoc.ID, permittableDoc); err != nil {
					return err
				}
			}
			return nil
		},
	})
}

// workspaceRolesEqual checks if two slices of WorkspaceRoleDocument are equal
func workspaceRolesEqual(a, b []mongodoc.WorkspaceRoleDocument) bool {
	if len(a) != len(b) {
		return false
	}

	// Create a map of workspace roles from slice a
	aMap := make(map[string]string)
	for _, wr := range a {
		aMap[wr.WorkspaceID] = wr.RoleID
	}

	// Check if all elements in b exist in a with same role
	for _, wr := range b {
		roleID, exists := aMap[wr.WorkspaceID]
		if !exists || roleID != wr.RoleID {
			return false
		}
	}

	return true
}
