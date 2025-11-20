package migration

import (
	"context"

	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateMissingWorkspaceAliases(ctx context.Context, c DBClient) error {
	col := c.Collection("workspace")

	// Query to find workspaces with problematic aliases or the specific eukarya workspace
	filter := bson.M{
		"$or": []bson.M{
			{"alias": ""},
			{"alias": "test"},
			{"alias": "aaaaa"},
			{"alias": "e2e-workspace-name"},
		},
	}

	return col.Find(ctx, filter, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc mongodoc.WorkspaceDocument

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}

				// Generate a random 10-character lowercase alias
				doc.Alias = random.String(10, random.Lowercase)

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			// Update all documents (they all need new aliases based on our query)
			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
