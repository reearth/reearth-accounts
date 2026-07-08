package migration

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

// BackfillAdminUserRole assigns role "system_admin" to every approved admin user
// that has no role yet, so existing approved admins keep full privileges once
// role enforcement is introduced.
func BackfillAdminUserRole(ctx context.Context, c DBClient) error {
	col := c.Database().Collection("adminuser")

	filter := bson.M{
		"status": "approved",
		"$or": []bson.M{
			{"role": bson.M{"$exists": false}},
			{"role": ""},
		},
	}
	update := bson.M{"$set": bson.M{"role": "system_admin"}}

	res, err := col.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to backfill adminuser.role: %w", err)
	}
	fmt.Printf("Backfilled adminuser.role=system_admin for %d approved users\n", res.ModifiedCount)
	return nil
}
