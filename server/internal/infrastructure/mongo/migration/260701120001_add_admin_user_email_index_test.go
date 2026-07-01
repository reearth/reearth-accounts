package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

func TestAddAdminUserEmailIndex_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	assert.NoError(t, AddAdminUserEmailIndex(ctx, mongoxClient))

	col := db.Collection("adminuser")

	_, err := col.InsertOne(ctx, mongodoc.AdminUserDocument{
		ID:     "admin1",
		Email:  "alice@eukarya.io",
		Name:   "Alice",
		Status: "pending",
	})
	assert.NoError(t, err, "first admin user should insert successfully")

	_, err = col.InsertOne(ctx, mongodoc.AdminUserDocument{
		ID:     "admin2",
		Email:  "ALICE@EUKARYA.IO", // same email, different case
		Name:   "Alice Dup",
		Status: "pending",
	})
	assert.Error(t, err, "duplicate case-different email should fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "should be duplicate key error")
}
