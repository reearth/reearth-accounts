package migration

import (
	"context"

	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateMissingWorkspaceAliases(ctx context.Context, c DBClient) error {
	col := c.Collection("workspace")
	
	// Test aliases that should be replaced
	testAliases := map[string]bool{
		"test":               true,
		"aaaaa":              true,
		"e2e-workspace-name": true,
	}

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

				needsNewAlias := false
				var newAlias string

				// Check if alias is missing (empty)
				if doc.Alias == "" {
					needsNewAlias = true
				}

				// Check if alias is one of the test aliases
				if testAliases[doc.Alias] {
					needsNewAlias = true
				}

				// Check for specific workspace that needs alias update
				if doc.ID == "01jhmkh59s3q06xzm1215w7y2v" && doc.Alias == "eukarya" {
					needsNewAlias = true
					newAlias = "eukarya-roboco"
				}

				if needsNewAlias {
					if newAlias != "" {
						doc.Alias = newAlias
					} else {
						doc.Alias = random.String(10, random.Lowercase)
					}
					
					ids = append(ids, doc.ID)
					newRows = append(newRows, doc)
				}
			}

			if len(newRows) > 0 {
				return col.SaveAll(ctx, ids, newRows)
			}
			
			return nil
		},
	})
}