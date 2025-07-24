package migration

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

type TempUserAlias struct {
	Alias string `validate:"required,printascii"`
}

func ConvertNonValidUserAlias(ctx context.Context, c DBClient) error {
	col := c.Collection("user")
	nameRegex := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-_@.]{0,61}[a-z0-9])?$`)

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

				doc.Alias = strings.ReplaceAll(doc.Alias, " ", "")
				doc.Alias = strings.ToLower(doc.Alias)

				// multiple consecutive characters check
				chars := []string{"-", "_", ".", "@"}
				for _, char := range chars {
					if strings.Contains(doc.Alias, char+char) {
						doc.Alias = strings.ReplaceAll(doc.Alias, char+char, char)
					}
				}

				// email address check
				if strings.Contains(doc.Alias, "@") && strings.Contains(doc.Alias, ".") {
					doc.Alias = random.String(10, random.Lowercase)
				}

				// validate alias against regex
				if !nameRegex.MatchString(doc.Alias) {
					doc.Alias = random.String(10, random.Lowercase)
				}

				var tempAlias TempUserAlias
				tempAlias.Alias = doc.Alias

				validate := validator.New()
				if err := validate.Struct(&tempAlias); err != nil {
					var invalidValidationError *validator.InvalidValidationError
					if errors.As(err, &invalidValidationError) {
						return err
					}

					doc.Alias = random.String(10, random.Lowercase)
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
