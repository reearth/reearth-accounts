package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddAdminUserEmailIndex creates a case-insensitive unique index on
// adminuser.email so an email can only ever belong to a single admin user.
func AddAdminUserEmailIndex(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("adminuser")

	emailIndexModel := mongo.IndexModel{
		Keys: map[string]interface{}{
			"email": 1,
		},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2, // Case-insensitive comparison
		}).SetUnique(true).SetName("adminuser_email_case_insensitive_unique"),
	}

	if _, err := col.Indexes().CreateOne(ctx, emailIndexModel); err != nil {
		return fmt.Errorf("failed to create unique index on adminuser.email: %w", err)
	}
	fmt.Println("Created unique case-insensitive index on adminuser.email")
	return nil
}
