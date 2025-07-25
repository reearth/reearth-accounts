package migration

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func RemoveMultipleWorkspaceOwners(ctx context.Context, c DBClient) error {
	col := c.Collection("workspace")

	return col.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc mongodoc.WorkspaceDocument

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}

				// Create map of user_id -> role for owners only
				owners := make(map[string]string)
				for userID, member := range doc.Members {
					if member.Role == string(workspace.RoleOwner) {
						owners[userID] = member.Role
					}
				}

				// Only process if there are multiple owners
				if len(owners) > 1 {
					needsUpdate := false
					for userID, member := range doc.Members {
						// Only change owners who were not self-invited to maintainer
						if member.Role == string(workspace.RoleOwner) && userID != member.InvitedBy {
							member.Role = string(workspace.RoleMaintainer)
							doc.Members[userID] = member
							needsUpdate = true
						}
					}

					// Only add to update list if changes were made
					if needsUpdate {
						ids = append(ids, doc.ID)
						newRows = append(newRows, doc)
					}
				}
			}

			// Only save if there are changes
			if len(ids) > 0 {
				return col.SaveAll(ctx, ids, newRows)
			}
			return nil
		},
	})
}
