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
1. The user must be an active (non-disabled) member of the workspace
2. The role must match the current role in workspace.members

Memory consideration:
This migration loads all workspace membership data into memory (expectedWorkspaceRoles map).
For large datasets (e.g., 10,000 workspaces with 10 members each = 100,000 entries),
this may consume significant memory. Each entry stores two strings (userID as key,
WorkspaceID + RoleID as value), roughly ~100 bytes per entry.
For most deployments this should be acceptable, but for very large datasets,
consider running this migration during off-peak hours or on a machine with adequate RAM.
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
					// Skip disabled members
					if member.Disabled {
						continue
					}

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

// workspaceRolesEqual checks if two slices of WorkspaceRoleDocument are equal.
// Note: This function assumes no duplicate WorkspaceIDs exist in either slice.
// In valid data, each user should have at most one role per workspace.
// If duplicates exist, the comparison may produce incorrect results.
func workspaceRolesEqual(a, b []mongodoc.WorkspaceRoleDocument) bool {
	if len(a) != len(b) {
		return false
	}

	if len(a) == 0 {
		return true
	}

	// Create a map of workspace roles from slice a
	// Key: "workspaceID:roleID" to handle potential duplicates correctly
	aSet := make(map[string]struct{})
	for _, wr := range a {
		key := wr.WorkspaceID + ":" + wr.RoleID
		aSet[key] = struct{}{}
	}

	// Check if all elements in b exist in a
	for _, wr := range b {
		key := wr.WorkspaceID + ":" + wr.RoleID
		if _, exists := aSet[key]; !exists {
			return false
		}
	}

	return true
}
