package migration

import (
	"context"
	"strings"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func AddMetadataUserV3(ctx context.Context, c DBClient) error {
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

				if doc.Alias == "" {
					alias := strings.ToLower(strings.ReplaceAll(doc.Name, " ", "-"))
					doc.Alias = alias
				}

				metadata := doc.Metadata

				if doc.Lang != "" {
					metadata.Lang = doc.Lang
					doc.Lang = ""
				}
				if doc.Theme != "" {
					metadata.Theme = doc.Theme
					doc.Theme = ""
				}

				doc.Metadata = metadata

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})

}
