// Package main provides MongoDB JSON Schema generation from Go struct definitions.
//
// This file contains the core generator logic that can be extracted to a separate
// library package for use in other projects.
//
// Usage as a library:
//
//	g := NewGenerator()
//	g.RegisterSchema("users", UserDocument{}, "User Schema", "Schema for user documents")
//	g.Generate(g.GetSchemas(), "./output")
//
// Or use RunCLI() for command-line interface:
//
//	g := NewGenerator()
//	g.RegisterSchema("users", UserDocument{}, "User Schema", "Schema for user documents")
//	g.RunCLI()
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
)

// SchemaConfig defines the configuration for generating a collection schema
type SchemaConfig struct {
	CollectionName string
	Type           any
	Title          string
	Description    string
	Required       []string
}

// Generator holds the type registry and generates schemas
type Generator struct {
	schemas []SchemaConfig
}

// NewGenerator creates a new schema generator
func NewGenerator() *Generator {
	return &Generator{
		schemas: make([]SchemaConfig, 0),
	}
}

// RegisterSchema registers a collection schema with its type and metadata
func (g *Generator) RegisterSchema(collection string, t any, title, description string) {
	g.schemas = append(g.schemas, SchemaConfig{
		CollectionName: collection,
		Type:           t,
		Title:          title,
		Description:    description,
	})
}

// GetSchemas returns all registered schemas
func (g *Generator) GetSchemas() []SchemaConfig {
	return g.schemas
}

// Generate generates schema files for all configured collections
func (g *Generator) Generate(schemas []SchemaConfig, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, cfg := range schemas {
		schema := g.generateMongoSchema(cfg)

		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema for %s: %w", cfg.CollectionName, err)
		}

		outputPath := filepath.Join(outputDir, cfg.CollectionName+".json")
		if err := os.WriteFile(outputPath, append(data, '\n'), 0644); err != nil {
			return fmt.Errorf("failed to write schema file %s: %w", outputPath, err)
		}

		fmt.Printf("Generated schema: %s\n", outputPath)
	}

	return nil
}

// RunCLI runs the generator as a command-line tool
func (g *Generator) RunCLI() {
	outputDir := flag.String("output", "./internal/infrastructure/mongo/schema", "Output directory for schema files")
	flag.Parse()

	schemas := g.GetSchemas()
	if len(schemas) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no schemas registered\n")
		os.Exit(1)
	}

	if err := g.Generate(schemas, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schemas: %v\n", err)
		os.Exit(1)
	}
}

func (g *Generator) generateMongoSchema(cfg SchemaConfig) map[string]any {
	r := &jsonschema.Reflector{
		DoNotReference: true,
		Anonymous:      true,
	}

	jsonSchema := r.Reflect(cfg.Type)
	mongoSchema := g.convertToMongoSchema(jsonSchema, cfg)

	return map[string]any{
		"$jsonSchema": mongoSchema,
	}
}

func (g *Generator) convertToMongoSchema(schema *jsonschema.Schema, cfg SchemaConfig) map[string]any {
	result := map[string]any{
		"bsonType":    "object",
		"title":       cfg.Title,
		"description": cfg.Description,
	}

	// Get required fields from struct tags if not specified in config
	required := cfg.Required
	if len(required) == 0 {
		required = getRequiredFieldsFromType(cfg.Type)
	}
	if len(required) > 0 {
		result["required"] = required
	}

	properties := make(map[string]any)

	// Add _id field
	properties["_id"] = map[string]any{
		"bsonType":    "objectId",
		"description": "MongoDB internal ID",
	}

	// Convert properties from JSON Schema
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			fieldName := toBsonFieldName(pair.Key)
			prop := pair.Value
			properties[fieldName] = g.convertPropertyToMongo(prop, cfg.Type, pair.Key)
		}
	}

	result["properties"] = properties
	result["additionalProperties"] = false

	return result
}

func (g *Generator) convertPropertyToMongo(prop *jsonschema.Schema, parentType any, originalFieldName string) map[string]any {
	result := make(map[string]any)

	// Get field info from struct for nullable detection
	isNullable := isPointerField(parentType, originalFieldName) || hasBsonOmitempty(parentType, originalFieldName)

	// Determine BSON type
	bsonType := determineBsonType(prop, parentType, originalFieldName)

	if isNullable {
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
		result["items"] = g.convertPropertyToMongo(prop.Items, nil, "")
	}

	// Handle nested object properties
	if bsonType == "object" && prop.Properties != nil {
		// Get the nested struct type for proper description lookup
		nestedType := getNestedType(parentType, originalFieldName)
		nestedProps := make(map[string]any)
		for pair := prop.Properties.Oldest(); pair != nil; pair = pair.Next() {
			nestedFieldName := toBsonFieldName(pair.Key)
			nestedProps[nestedFieldName] = g.convertPropertyToMongo(pair.Value, nestedType, pair.Key)
		}
		result["properties"] = nestedProps
	}

	// Handle map types (additionalProperties) - only for true map types without Properties
	if prop.AdditionalProperties != nil && prop.Properties == nil {
		// Get the map value type for proper description lookup
		mapValueType := getMapValueType(parentType, originalFieldName)
		result["additionalProperties"] = g.convertPropertyToMongo(prop.AdditionalProperties, mapValueType, "")
	}

	return result
}

func determineBsonType(prop *jsonschema.Schema, parentType any, fieldName string) string {
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
	// The fieldName comes from jsonschema library which uses the json tag value.
	// We use it as-is since json tags should match bson field names.
	return fieldName
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

func getNestedType(parentType any, jsonFieldName string) any {
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

func getMapValueType(parentType any, jsonFieldName string) any {
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

func isPointerField(parentType any, jsonFieldName string) bool {
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

func hasBsonOmitempty(parentType any, jsonFieldName string) bool {
	if parentType == nil || jsonFieldName == "" {
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

	bsonTag := field.Tag.Get("bson")
	return strings.Contains(bsonTag, "omitempty")
}

func isTimeField(parentType any, jsonFieldName string) bool {
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

func isByteSlice(parentType any, jsonFieldName string) bool {
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

func getDescriptionFromTag(parentType any, jsonFieldName string) string {
	return getTagValue(parentType, jsonFieldName, "description")
}

func getTagValue(parentType any, jsonFieldName string, key string) string {
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

	// Parse value from jsonschema tag
	// Format: jsonschema:"key=value,key2=value2" or jsonschema:"required,description=value"
	prefix := key + "="
	idx := strings.Index(tag, prefix)
	if idx == -1 {
		return ""
	}

	value := tag[idx+len(prefix):]
	// Value continues until the next comma or end of tag
	// But for "description=", it continues to the end (description is always last)
	if key != "description" {
		if commaIdx := strings.Index(value, ","); commaIdx != -1 {
			value = value[:commaIdx]
		}
	}

	return value
}

func getRequiredFieldsFromType(t any) []string {
	if t == nil {
		return nil
	}

	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
	}

	var required []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		// Get JSON field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == "" {
			continue
		}

		// Check for jsonschema:"required" tag
		jsonschemaTag := field.Tag.Get("jsonschema")
		if jsonschemaTag == "" {
			continue
		}

		// Parse comma-separated parts to find "required"
		parts := strings.Split(jsonschemaTag, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "required" {
				required = append(required, jsonName)
				break
			}
		}
	}

	return required
}
