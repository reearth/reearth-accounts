package migration

import (
	"context"
	"errors"
	"strings"

	validator "github.com/go-playground/validator/v10"
	"github.com/labstack/gommon/random"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

type TempASCIIWorkspaceAlias struct {
	Alias string `validate:"required,printascii"`
}

func ConvertNonASCIIWorkspaceAlias(ctx context.Context, c DBClient) error {
	col := c.Collection("workspace")

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

				doc.Alias = strings.ReplaceAll(doc.Alias, " ", "")

				var tempAlias TempASCIIWorkspaceAlias
				tempAlias.Alias = doc.Alias

				// Validate the alias to ensure it is ASCII
				validate := validator.New()
				if err := validate.Struct(&tempAlias); err != nil {
					var invalidValidationError *validator.InvalidValidationError
					if errors.As(err, &invalidValidationError) {
						return err
					}

					// If the alias is not valid ASCII, generate a new random alias
					doc.Alias = strings.ToLower(random.String(10))
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
