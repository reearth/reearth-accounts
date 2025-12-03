package migration

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/oklog/ulid"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

/*
Move workspace.members.role to permittable.workspace_roles. workspace_roles is an array of objects
containing workspace_id and role_id from role collection.
*/

func MoveWorkspaceMembersRoleToPermittable(ctx context.Context, c DBClient) error {
	roleCol := c.Collection("role")
	permittableCol := c.Collection("permittable")
	workspaceCol := c.Collection("workspace")

	// Step 1: Load all roles and create a map from role name to role ID
	roleNameToID := make(map[string]string)
	err := roleCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var roleDoc mongodoc.RoleDocument
				if err := bson.Unmarshal(row, &roleDoc); err != nil {
					return err
				}
				roleNameToID[roleDoc.Name] = roleDoc.ID
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Step 2: Load all existing permittables and index by UserID
	permittablesByUserID := make(map[string]*mongodoc.PermittableDocument)
	err = permittableCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var permittableDoc mongodoc.PermittableDocument
				if err = bson.Unmarshal(row, &permittableDoc); err != nil {
					return err
				}
				permittablesByUserID[permittableDoc.UserID] = &permittableDoc
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Step 3: Process all workspaces and build workspace roles
	err = workspaceCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var wsDoc mongodoc.WorkspaceDocument
				if err = bson.Unmarshal(row, &wsDoc); err != nil {
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

					permittableDoc, exists := permittablesByUserID[userIDStr]
					if !exists {
						// Create new permittable
						newPermittableID := strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())
						permittableDoc = &mongodoc.PermittableDocument{
							ID:             newPermittableID,
							UserID:         userIDStr,
							RoleIDs:        []string{roleNameToID[interfaces.RoleSelf]},
							WorkspaceRoles: []mongodoc.WorkspaceRoleDocument{workspaceRole},
						}
						permittablesByUserID[userIDStr] = permittableDoc
					} else {
						// Set Role ID to "self" if not already present
						roleIdsExists := false
						for _, rid := range permittableDoc.RoleIDs {
							if rid == roleNameToID[interfaces.RoleSelf] {
								roleIdsExists = true
								break
							}
						}

						if !roleIdsExists {
							permittableDoc.RoleIDs = append(permittableDoc.RoleIDs, roleNameToID[interfaces.RoleSelf])
						}

						// Check if this workspace role already exists
						roleExists := false
						for _, wr := range permittableDoc.WorkspaceRoles {
							if wr.WorkspaceID == wsDoc.ID && wr.RoleID == roleID {
								roleExists = true
								break
							}
						}

						if !roleExists {
							permittableDoc.WorkspaceRoles = append(permittableDoc.WorkspaceRoles, workspaceRole)
						}
					}
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Step 4: Save all updated/created permittables
	for _, permittableDoc := range permittablesByUserID {
		if err = permittableCol.SaveOne(ctx, permittableDoc.ID, permittableDoc); err != nil {
			return err
		}
	}

	return nil
}
