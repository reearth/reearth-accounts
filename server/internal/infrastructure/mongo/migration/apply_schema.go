package migration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/schema"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ApplyCollectionSchemas creates collections with JSON schema validators if they don't exist,
// or updates existing collections with the schema validators.
// Schemas are read from embedded JSON files in the schema package.
func ApplyCollectionSchemas(ctx context.Context, collections []string, c DBClient) error {
	db := c.Database()

	for _, collName := range collections {
		schemaData, err := schema.SchemaFS.ReadFile(collName + ".json")
		if err != nil {
			return fmt.Errorf("failed to read schema file for %s: %w", collName, err)
		}

		var schemaDoc bson.M
		if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
			return fmt.Errorf("failed to parse schema JSON for %s: %w", collName, err)
		}

		if err := applySchema(ctx, db, collName, schemaDoc); err != nil {
			return fmt.Errorf("failed to apply schema for collection %s: %w", collName, err)
		}
		fmt.Printf("Applied schema for collection: %s\n", collName)
	}

	return nil
}

func applySchema(ctx context.Context, db *mongo.Database, collName string, schema bson.M) error {
	// Check if collection exists
	collections, err := db.ListCollectionNames(ctx, bson.M{"name": collName})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	if len(collections) == 0 {
		// Collection doesn't exist, create it with the schema (strict validation for new collections)
		opts := options.CreateCollection().
			SetValidator(schema).
			SetValidationLevel("strict").
			SetValidationAction("error")
		if err := db.CreateCollection(ctx, collName, opts); err != nil {
			return fmt.Errorf("failed to create collection %s: %w", collName, err)
		}
		fmt.Printf("Created collection: %s\n", collName)
	} else {
		// Collection exists, update the validator
		cmd := bson.D{
			{Key: "collMod", Value: collName},
			{Key: "validator", Value: schema},
			{Key: "validationLevel", Value: "moderate"},
			{Key: "validationAction", Value: "error"},
		}
		if err := db.RunCommand(ctx, cmd).Err(); err != nil {
			return fmt.Errorf("failed to update validator for collection %s: %w", collName, err)
		}
		fmt.Printf("Updated validator for collection: %s\n", collName)
	}

	return nil
}
