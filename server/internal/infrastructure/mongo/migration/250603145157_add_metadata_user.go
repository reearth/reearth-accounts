package migration

import (
	"context"
	"strings"
	"time"

	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

// NOTE:
// The original `UserDocument` structs from `mongodoc` were updated to remove the pointer from the `Metadata` field.
// As a result, keeping the original migration logic caused compilation errors.
// To maintain compatibility and avoid coupling this migration to future struct changes, we define local legacy versions of the structs below, matching the schema at the time this migration was first written.

// userDocumentLegacy represents the old version of UserDocument
type userDocumentLegacy struct {
	ID            string
	Name          string
	Alias         string
	Email         string
	Subs          []string
	Workspace     string
	Team          string `bson:",omitempty"`
	Lang          string
	Theme         string
	Password      []byte
	PasswordReset *passwordResetDocumentLegacy
	Verification  *userVerificationDocLegacy
	Metadata      *userMetadataDocLegacy
}

// userVerificationDoc represents the old version of UserVerificationDoc
type passwordResetDocumentLegacy struct {
	Token     string
	CreatedAt time.Time
}

// userVerificationDocLegacy represents the old version of UserVerificationDoc
type userVerificationDocLegacy struct {
	Code       string
	Expiration time.Time
	Verified   bool
}

// userMetadataDocLegacy represents the old version of UserMetadataDoc
type userMetadataDocLegacy struct {
	Description string
	Website     string
	PhotoURL    string
	Lang        string
	Theme       string
}

// AddMetadataUser is a legacy migration to initialize metadata fields in user documents.
// This version avoids using updated mongodoc structs to prevent compile issues.
func AddMetadataUser(ctx context.Context, c DBClient) error {
	col := c.Collection("user")

	return col.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc userDocumentLegacy
				metadata := new(userMetadataDocLegacy)

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}
				if doc.Alias == "" {
					alias := strings.ToLower(strings.ReplaceAll(doc.Name, " ", "-"))
					doc.Alias = alias
				}

				metadata := doc.Metadata

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

					doc.Metadata = &userMetadataDocLegacy{
						Description: "",
						Lang:        lang,
						PhotoURL:    "",
						Theme:       theme,
						Website:     "",
					}
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
