package migration

import (
	"context"
	"log"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func SyncUserNameToWorkspace(ctx context.Context, c DBClient) error {
	userCol := c.Collection("user")
	workspaceCol := c.Collection("workspace")

	// Build a map of workspace ID to user name
	userNameMap := make(map[string]string)

	// First, get all users and map their workspace to name
	err := userCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var userDoc mongodoc.UserDocument
				if err := bson.Unmarshal(row, &userDoc); err != nil {
					return err
				}
				// Map workspace ID to user name
				if userDoc.Workspace != "" && userDoc.Name != "" {
					userNameMap[userDoc.Workspace] = userDoc.Name
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	// Now update personal workspaces where name matches email format using regex
	// Use MongoDB regex filter to retrieve only workspaces with email-formatted names
	filter := bson.M{
		"personal": true,
		"name": bson.M{
			"$regex":   `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
			"$options": "i",
		},
	}

	return workspaceCol.Find(ctx, filter, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]any, 0, len(rows))

			for _, row := range rows {
				var wsDoc mongodoc.WorkspaceDocument
				if err := bson.Unmarshal(row, &wsDoc); err != nil {
					return err
				}

				// Check if we have a user name for this workspace
				userName, ok := userNameMap[wsDoc.ID]
				if !ok || userName == "" {
					continue
				}

				// Only update if names differ
				if wsDoc.Name != userName {
					log.Println("updating workspace name:", wsDoc.ID, "from", maskEmail(wsDoc.Name), "to", userName)
					wsDoc.Name = userName
					ids = append(ids, wsDoc.ID)
					newRows = append(newRows, wsDoc)
				}
			}

			log.Println("count of updated workspaces:", len(ids))

			// Only save if there are records to update
			if len(ids) > 0 {
				return workspaceCol.SaveAll(ctx, ids, newRows)
			}
			return nil
		},
	})
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	localPart := parts[0]
	domain := parts[1]

	if len(localPart) == 0 {
		return email
	}

	if len(localPart) == 1 {
		return localPart[0:1] + "***@" + domain
	}

	return localPart[0:1] + "***@" + domain
}
