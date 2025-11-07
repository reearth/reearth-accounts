package migration

import (
	"context"
	"log"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func SyncPersonalWorkspaceAlias(ctx context.Context, c DBClient) error {
	userCol := c.Collection("user")
	workspaceCol := c.Collection("workspace")

	// Build a map of workspace ID to user alias
	userAliasMap := make(map[string]string)

	// First, get all users and map their workspace to alias
	err := userCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var userDoc mongodoc.UserDocument
				if err := bson.Unmarshal(row, &userDoc); err != nil {
					return err
				}
				// Map workspace ID to user alias
				if userDoc.Workspace != "" && userDoc.Alias != "" {
					userAliasMap[userDoc.Workspace] = userDoc.Alias
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Now update workspaces where personal=true and alias differs from user's alias
	return workspaceCol.Find(ctx, bson.M{"personal": true}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var wsDoc mongodoc.WorkspaceDocument
				if err := bson.Unmarshal(row, &wsDoc); err != nil {
					return err
				}

				// Check if we have a user alias for this workspace
				if userAlias, ok := userAliasMap[wsDoc.ID]; ok {
					// Only update if aliases differ
					if wsDoc.Alias != userAlias {
						wsDoc.Alias = userAlias
						ids = append(ids, wsDoc.ID)
						newRows = append(newRows, wsDoc)
					}
				}
			}

			log.Println("count of ids: ", len(ids))

			// Only save if there are records to update
			if len(ids) > 0 {
				return workspaceCol.SaveAll(ctx, ids, newRows)
			}
			return nil
		},
	})
}
