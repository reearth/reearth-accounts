package migration

import (
	"context"
	"strings"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func AddMetadataUser(ctx context.Context, c DBClient) error {
	col := c.Collection("user")

	return col.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc mongodoc.UserDocument
				metadata := new(mongodoc.UserMetadataDocument)

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}

				if doc.Alias == "" {
					alias := strings.ToLower(strings.ReplaceAll(doc.Name, " ", "-"))
					doc.Alias = alias
				}

				if doc.Metadata != nil {
					if doc.Lang != "" {
						metadata.Lang = doc.Lang
						doc.Lang = ""
					}

					if doc.Theme != "" {
						metadata.Theme = doc.Theme
						doc.Theme = ""
					}

					metadata.Description = doc.Metadata.Description
					metadata.PhotoURL = doc.Metadata.PhotoURL
					metadata.Website = doc.Metadata.Website
					doc.Metadata = metadata
				} else {
					var lang, theme string
					if doc.Lang != "" {
						lang = doc.Lang
						doc.Lang = ""
					}

					if doc.Theme != "" {
						theme = doc.Theme
						doc.Theme = ""
					}

					doc.Metadata = &mongodoc.UserMetadataDocument{
						Description: "",
						Lang:        lang,
						PhotoURL:    "",
						Theme:       theme,
						Website:     "",
					}
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})

}
