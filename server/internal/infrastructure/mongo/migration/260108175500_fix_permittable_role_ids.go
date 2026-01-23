package migration

import (
	"context"
	"slices"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/mongox"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/bson"
)

func FixPermittableRoleIDs(ctx context.Context, c DBClient) error {
	roleCol := c.Collection("role")
	permittableCol := c.Collection("permittable")

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

	// Identify IDs to remove and "self" ID
	// Filter out empty strings to avoid unintended removal of legitimate values
	var rolesToRemove []string
	for _, roleName := range []string{
		string(role.RoleOwner),
		string(role.RoleMaintainer),
		string(role.RoleWriter),
		string(role.RoleReader),
	} {
		if roleID, exists := roleNameToID[roleName]; exists && roleID != "" {
			rolesToRemove = append(rolesToRemove, roleID)
		}
	}
	selfRoleID := roleNameToID[interfaces.RoleSelf]

	// Step 2: Iterate all permittables and fix RoleIDs
	return permittableCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var permittableDoc mongodoc.PermittableDocument
				if err := bson.Unmarshal(row, &permittableDoc); err != nil {
					return err
				}

				originalRoleIDs := make([]string, len(permittableDoc.RoleIDs))
				copy(originalRoleIDs, permittableDoc.RoleIDs)

				// Filter out workspace roles and empty strings
				newRoleIDs := lo.Filter(permittableDoc.RoleIDs, func(id string, _ int) bool {
					return id != "" && !lo.Contains(rolesToRemove, id)
				})

				// Ensure "self" role exists
				if selfRoleID != "" && !lo.Contains(newRoleIDs, selfRoleID) {
					newRoleIDs = append(newRoleIDs, selfRoleID)
				}

				// Update if changed
				if !slices.Equal(originalRoleIDs, newRoleIDs) {
					permittableDoc.RoleIDs = newRoleIDs
					if err := permittableCol.SaveOne(ctx, permittableDoc.ID, permittableDoc); err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
}
