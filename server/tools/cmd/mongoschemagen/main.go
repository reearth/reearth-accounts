// schemagen generates MongoDB $jsonSchema from Go struct definitions using invopop/jsonschema.
//
// Usage:
//
//	go run ./tools/cmd/schemagen -output ./internal/infrastructure/mongo/schema
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
)

// SchemaConfig defines the configuration for generating a collection schema
type SchemaConfig struct {
	CollectionName string
	Type           interface{}
	Title          string
	Description    string
	Required       []string
}

var schemas = []SchemaConfig{
	{
		CollectionName: "user",
		Type:           mongodoc.UserDocument{},
		Title:          "User Collection Schema",
		Description:    "Schema for user documents in the reearth-accounts database",
		Required:       []string{"id", "name", "alias", "email", "subs", "workspace", "metadata", "password"},
	},
	{
		CollectionName: "workspace",
		Type:           mongodoc.WorkspaceDocument{},
		Title:          "Workspace Collection Schema",
		Description:    "Schema for workspace documents in the reearth-accounts database",
		Required:       []string{"id", "name", "alias", "metadata", "email", "members", "personal"},
	},
	{
		CollectionName: "role",
		Type:           mongodoc.RoleDocument{},
		Title:          "Role Collection Schema",
		Description:    "Schema for role documents in the reearth-accounts database",
		Required:       []string{"id", "name"},
	},
	{
		CollectionName: "permittable",
		Type:           mongodoc.PermittableDocument{},
		Title:          "Permittable Collection Schema",
		Description:    "Schema for permittable documents in the reearth-accounts database",
		Required:       []string{"id", "userid"},
	},
	{
		CollectionName: "config",
		Type:           mongodoc.ConfigDocument{},
		Title:          "Config Collection Schema",
		Description:    "Schema for config documents in the reearth-accounts database",
		Required:       []string{},
	},
}

func main() {
	outputDir := flag.String("output", "./internal/infrastructure/mongo/schema", "Output directory for schema files")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	for _, cfg := range schemas {
		schema := generateMongoSchema(cfg)

		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling schema for %s: %v\n", cfg.CollectionName, err)
			os.Exit(1)
		}

		outputPath := filepath.Join(*outputDir, cfg.CollectionName+".json")
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing schema file %s: %v\n", outputPath, err)
			os.Exit(1)
		}

		fmt.Printf("Generated schema: %s\n", outputPath)
	}
}

func generateMongoSchema(cfg SchemaConfig) map[string]interface{} {
	r := &jsonschema.Reflector{
		DoNotReference: true,
		Anonymous:      true,
	}

	jsonSchema := r.Reflect(cfg.Type)
	mongoSchema := convertToMongoSchema(jsonSchema, cfg)

	return map[string]interface{}{
		"$jsonSchema": mongoSchema,
	}
}

func convertToMongoSchema(schema *jsonschema.Schema, cfg SchemaConfig) map[string]interface{} {
	result := map[string]interface{}{
		"bsonType":    "object",
		"title":       cfg.Title,
		"description": cfg.Description,
	}

	if len(cfg.Required) > 0 {
		result["required"] = cfg.Required
	}

	properties := make(map[string]interface{})

	// Add _id field
	properties["_id"] = map[string]interface{}{
		"bsonType":    "objectId",
		"description": "MongoDB internal ID",
	}

	// Convert properties from JSON Schema
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			fieldName := toBsonFieldName(pair.Key)
			prop := pair.Value
			properties[fieldName] = convertPropertyToMongo(prop, cfg.Type, pair.Key)
		}
	}

	result["properties"] = properties
	result["additionalProperties"] = false

	return result
}

func convertPropertyToMongo(prop *jsonschema.Schema, parentType interface{}, originalFieldName string) map[string]interface{} {
	result := make(map[string]interface{})

	// Get field info from struct for nullable detection
	isPointer := isPointerField(parentType, originalFieldName)

	// Determine BSON type
	bsonType := determineBsonType(prop, parentType, originalFieldName)

	if isPointer {
		result["bsonType"] = []string{bsonType, "null"}
	} else {
		result["bsonType"] = bsonType
	}

	// Get description from struct tag (more reliable than prop.Description which truncates on commas)
	description := getDescriptionFromTag(parentType, originalFieldName)
	if description == "" && prop.Description != "" {
		description = prop.Description
	}
	if description != "" {
		result["description"] = description
	}

	// Handle array items
	if bsonType == "array" && prop.Items != nil {
		result["items"] = convertPropertyToMongo(prop.Items, nil, "")
	}

	// Handle nested object properties
	if bsonType == "object" && prop.Properties != nil {
		// Get the nested struct type for proper description lookup
		nestedType := getNestedType(parentType, originalFieldName)
		nestedProps := make(map[string]interface{})
		for pair := prop.Properties.Oldest(); pair != nil; pair = pair.Next() {
			nestedFieldName := toBsonFieldName(pair.Key)
			nestedProps[nestedFieldName] = convertPropertyToMongo(pair.Value, nestedType, pair.Key)
		}
		result["properties"] = nestedProps

		// Only add additionalProperties for map types (not nested structs)
		// Nested structs have Properties defined, map types have AdditionalProperties
		if prop.AdditionalProperties == nil {
			// This is a struct, don't add additionalProperties
		}
	}

	// Handle map types (additionalProperties) - only for true map types without Properties
	if prop.AdditionalProperties != nil && prop.Properties == nil {
		// Get the map value type for proper description lookup
		mapValueType := getMapValueType(parentType, originalFieldName)
		result["additionalProperties"] = convertPropertyToMongo(prop.AdditionalProperties, mapValueType, "")
	}

	return result
}

func determineBsonType(prop *jsonschema.Schema, parentType interface{}, fieldName string) string {
	if prop == nil {
		return "string"
	}

	// Check for time.Time
	if isTimeField(parentType, fieldName) {
		return "date"
	}

	// Check for []byte
	if isByteSlice(parentType, fieldName) {
		return "binData"
	}

	// Handle types from JSON Schema
	types := prop.Type
	if types == "" && len(prop.AnyOf) > 0 {
		// Handle anyOf for nullable types
		for _, anyOf := range prop.AnyOf {
			if anyOf.Type != "" && anyOf.Type != "null" {
				types = anyOf.Type
				break
			}
		}
	}

	switch types {
	case "string":
		if prop.Format == "date-time" {
			return "date"
		}
		return "string"
	case "integer":
		return "long"
	case "number":
		return "double"
	case "boolean":
		return "bool"
	case "array":
		return "array"
	case "object":
		return "object"
	default:
		return "string"
	}
}

func toBsonFieldName(fieldName string) string {
	// Convert to lowercase for BSON field naming convention
	return strings.ToLower(fieldName)
}

func findFieldByJSONName(t reflect.Type, jsonFieldName string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		jsonTag := f.Tag.Get("json")
		// Handle json tag like "fieldname" or "fieldname,omitempty"
		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == jsonFieldName {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

func getNestedType(parentType interface{}, jsonFieldName string) interface{} {
	if parentType == nil {
		return nil
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	// If no field name is provided, return the parentType itself
	// This handles cases like additionalProperties where parentType is already the nested type
	if jsonFieldName == "" {
		return parentType
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return nil
	}

	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Struct {
		return reflect.New(fieldType).Elem().Interface()
	}

	return nil
}

func getMapValueType(parentType interface{}, jsonFieldName string) interface{} {
	if parentType == nil || jsonFieldName == "" {
		return nil
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return nil
	}

	fieldType := field.Type
	if fieldType.Kind() == reflect.Map {
		valueType := fieldType.Elem()
		if valueType.Kind() == reflect.Struct {
			return reflect.New(valueType).Elem().Interface()
		}
	}

	return nil
}

func isPointerField(parentType interface{}, jsonFieldName string) bool {
	if parentType == nil {
		return false
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return false
	}

	return field.Type.Kind() == reflect.Ptr
}

func isTimeField(parentType interface{}, jsonFieldName string) bool {
	if parentType == nil {
		return false
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return false
	}

	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	return fieldType == reflect.TypeOf(time.Time{})
}

func isByteSlice(parentType interface{}, jsonFieldName string) bool {
	if parentType == nil {
		return false
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return false
	}

	return field.Type == reflect.TypeOf([]byte{})
}

func getDescriptionFromTag(parentType interface{}, jsonFieldName string) string {
	if parentType == nil || jsonFieldName == "" {
		return ""
	}

	t := reflect.TypeOf(parentType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}

	field, found := findFieldByJSONName(t, jsonFieldName)
	if !found {
		return ""
	}

	tag := field.Tag.Get("jsonschema")
	if tag == "" {
		return ""
	}

	// Parse description from jsonschema tag
	// Format: jsonschema:"description=Some description here"
	const prefix = "description="
	if idx := strings.Index(tag, prefix); idx != -1 {
		desc := tag[idx+len(prefix):]
		// The description continues until the end of the tag value
		return desc
	}

	return ""
}
