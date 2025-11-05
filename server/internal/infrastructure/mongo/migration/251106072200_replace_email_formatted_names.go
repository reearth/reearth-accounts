package migration

import (
	"context"
	"regexp"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ReplaceEmailFormattedNames(ctx context.Context, c DBClient) error {
	col := c.Collection("user")
	seenNames := make(map[string]bool)

	// Query to find users with email-formatted names (contains @ symbol)
	filter := bson.M{
		"name": bson.M{
			"$regex": "@",
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

				// Double-check with strict email regex
				if emailRegex.MatchString(doc.Name) {
					newName := generateUniqueName(seenNames)
					doc.Name = newName
					seenNames[newName] = true

					ids = append(ids, doc.ID)
					newRows = append(newRows, doc)
				}
			}

			// Only save if there are records to update
			if len(ids) > 0 {
				return col.SaveAll(ctx, ids, newRows)
			}
			return nil
		},
	})
}

func generateUniqueName(seenNames map[string]bool) string {
	return "user-" + id.NewUserID().String()
}
