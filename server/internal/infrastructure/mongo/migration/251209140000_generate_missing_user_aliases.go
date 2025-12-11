package migration

import (
	"context"

	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateMissingUserAliases(ctx context.Context, c DBClient) error {
	col := c.Collection("user")

	// Query to find users with empty alias or alias equal to "waqas"
	filter := bson.M{
		"$or": []bson.M{
			{"alias": ""},
			{"alias": "waqas"},
		},
	}

	return col.Find(ctx, filter, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc mongodoc.UserDocument

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}

				// All returned documents need new aliases (due to our query filter)
				alias := random.String(10, random.Lowercase)
				if doc.Name != "" && doc.ID != "" {
					alias = doc.Name + doc.ID
				}
				doc.Alias = alias

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			// Update all documents (they all need new aliases based on our query)
			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
