package migration

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/oklog/ulid"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

func AddRoles(ctx context.Context, c DBClient) error {
	roleCol := c.Collection("role")
	return roleCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: 1000,
		Callback: func(rows []bson.Raw) error {
			roleToAdd := []string{"reader", "writer", "maintainer", "owner", "self"}
			existingRoles := make(map[string]bool)
			for _, row := range rows {
				var roleDoc mongodoc.RoleDocument
				if err := bson.Unmarshal(row, &roleDoc); err != nil {
					return err
				}
				existingRoles[roleDoc.Name] = true
			}

			for _, roleName := range roleToAdd {
				if !existingRoles[roleName] {
					id := strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())
					err := roleCol.SaveOne(ctx, id, &mongodoc.RoleDocument{
						ID:   id,
						Name: roleName,
					})
					if err != nil {
						return err
					}
				}
			}

			return nil
		},
	})
}
