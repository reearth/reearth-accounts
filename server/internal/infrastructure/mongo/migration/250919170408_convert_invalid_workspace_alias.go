package migration

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

type TempWorkspaceAliasNew struct {
	Alias string `validate:"required,min=5,max=30,printascii"`
}

func ConvertInvalidWorkspaceAlias(ctx context.Context, c DBClient) error {
	col := c.Collection("workspace")
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{3,30}[a-zA-Z0-9]$`)

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

				originalAlias := doc.Alias
				sanitizedAlias := sanitizeAlias(originalAlias)

				// Apply sanitization if original doesn't match regex or has consecutive hyphens
				if !nameRegex.MatchString(originalAlias) || strings.Contains(originalAlias, "--") {
					doc.Alias = sanitizedAlias
				}

				var tempAlias TempWorkspaceAliasNew
				tempAlias.Alias = doc.Alias

				validate := validator.New()
				if err := validate.Struct(&tempAlias); err != nil {
					var invalidValidationError *validator.InvalidValidationError
					if errors.As(err, &invalidValidationError) {
						return err
					}

					// If validation still fails after sanitization, try again
					doc.Alias = sanitizeAlias(doc.Alias)
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
