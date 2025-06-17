package migration

import (
	"context"
	"strings"

	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

// NOTE:
// The original `WorkspaceDocument` structs from `mongodoc` were updated to remove the pointer from the `Metadata` field.
// As a result, keeping the original migration logic caused compilation errors.
// To maintain compatibility and avoid coupling this migration to future struct changes, we define local legacy versions of the structs below, matching the schema at the time this migration was first written.

// workspaceDocumentLegacy represents the old version of WorkspaceDocument
type workspaceDocumentLegacy struct {
	ID           string
	Name         string
	Alias        string
	Email        string
	Metadata     *workspaceMetadataDocumentLegacy
	Members      map[string]workspaceMemberDocumentLegacy
	Integrations map[string]workspaceMemberDocumentLegacy
	Personal     bool
	Policy       string `bson:",omitempty"`
}

// workspaceMetadataDocumentLegacy represents the old version of WorkspaceMetadataDocument
type workspaceMetadataDocumentLegacy struct {
	Description  string
	Website      string
	Location     string
	BillingEmail string
	PhotoURL     string
}

// workspaceMemberDocumentLegacy represents the old version of WorkspaceMemberDocument
type workspaceMemberDocumentLegacy struct {
	Role      string
	InvitedBy string
	Disabled  bool
}

func AddMetadataWorkspace(ctx context.Context, c DBClient) error {
	col := c.WithCollection("workspace")

	return col.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			ids := make([]string, 0, len(rows))
			newRows := make([]interface{}, 0, len(rows))

			for _, row := range rows {
				var doc workspaceDocumentLegacy
				metadata := new(workspaceMetadataDocumentLegacy)

				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}
				if doc.Email == "" {
					doc.Email = ""
				}
				if doc.Alias == "" {
					alias := strings.ToLower(strings.ReplaceAll(doc.Name, " ", "-"))
					doc.Alias = alias
				}

				if doc.Metadata != nil {
					metadata.BillingEmail = doc.Metadata.BillingEmail
					metadata.Description = doc.Metadata.Description
					metadata.Location = doc.Metadata.Location
					metadata.PhotoURL = doc.Metadata.PhotoURL
					metadata.Website = doc.Metadata.Website

					doc.Metadata = metadata
				} else {
					doc.Metadata = metadata
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}
