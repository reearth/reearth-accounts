package migration

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func AlterPermittableRoleIdsToUserRoles(ctx context.Context, c DBClient) error {
	permittableCol := c.Collection("permittable")

	err := permittableCol.UpdateMany(
		ctx,
		bson.D{},
		bson.D{
			{Key: "$rename", Value: bson.D{
				{Key: "roleids", Value: "user_roles"},
			}},
		},
	)

	return err
}
