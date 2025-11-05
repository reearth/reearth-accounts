package migration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ReplaceEmailFormattedNames(ctx context.Context, c DBClient) error {
	col := c.Collection("user")
	seenNames := make(map[string]bool)

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

				// Check if name is in email format
				if emailRegex.MatchString(doc.Name) {
					newName := generateUniqueName(seenNames)
					doc.Name = newName
					seenNames[newName] = true
				} else {
					// Track non-email names to avoid collisions
					seenNames[doc.Name] = true
				}

				ids = append(ids, doc.ID)
				newRows = append(newRows, doc)
			}

			return col.SaveAll(ctx, ids, newRows)
		},
	})
}

func generateUniqueName(seenNames map[string]bool) string {
	for {
		randomStr := generateRandomString(8)
		name := "user-" + randomStr
		if !seenNames[name] {
			return name
		}
	}
}

func generateRandomString(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based approach if random fails
		return strings.ToLower(hex.EncodeToString([]byte("fallback"))[:length])
	}
	return hex.EncodeToString(bytes)[:length]
}
