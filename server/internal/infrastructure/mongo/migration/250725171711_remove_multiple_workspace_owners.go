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

				ownerCount := 0
				for _, member := range doc.Members {
					if member.Role == string(workspace.RoleOwner) {
						ownerCount++
					}
				}

				for userID, member := range doc.Members {
					if member.Role == string(workspace.RoleOwner) && ownerCount > 1 {
						member.Role = string(workspace.RoleMaintainer)
						doc.Members[userID] = member
						ownerCount--
					}
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
