package migration

import (
	"context"

	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateMissingUserAliases(ctx context.Context, c DBClient) error {
	col := c.Collection("user")

	return col.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc mongodoc.UserDocument

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}

				needsNewAlias := false

				// Check if alias is missing (empty)
				if doc.Alias == "" {
					needsNewAlias = true
				}

				// Check if alias is "waqas"
				if doc.Alias == "waqas" {
					needsNewAlias = true
				}

				if needsNewAlias {
					// Generate a random 10-character lowercase alias
					doc.Alias = random.String(10, random.Lowercase)
					
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