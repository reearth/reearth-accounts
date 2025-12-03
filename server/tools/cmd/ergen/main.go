// ergen generates an ER diagram in Mermaid format from MongoDB JSON schema files.
//
// Usage:
//
//	go run ./tools/cmd/ergen -schema ./internal/infrastructure/mongo/schema -output ./internal/infrastructure/mongo/schema/ER.md
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type JSONSchema struct {
	Schema *SchemaDefinition `json:"$jsonSchema"`
}

type SchemaDefinition struct {
	BsonType    interface{}                  `json:"bsonType"`
	Title       string                       `json:"title"`
	Description string                       `json:"description"`
	Required    []string                     `json:"required"`
	Properties  map[string]PropertyDefinition `json:"properties"`
}

type PropertyDefinition struct {
	BsonType             interface{}                   `json:"bsonType"`
	Description          string                        `json:"description"`
	Items                *PropertyDefinition           `json:"items"`
	Properties           map[string]PropertyDefinition `json:"properties"`
	AdditionalProperties *PropertyDefinition           `json:"additionalProperties"`
}

type Collection struct {
	Name       string
	Properties []Property
	Required   map[string]bool
}

type Property struct {
	Name        string
	Type        string
	Description string
	IsRequired  bool
	IsPK        bool
	IsFK        bool
	FKRef       string
}

func main() {
	schemaDir := flag.String("schema", "./internal/infrastructure/mongo/schema", "Path to schema directory")
	outputFile := flag.String("output", "", "Output file path (stdout if not specified)")
	flag.Parse()

	collections, err := loadSchemas(*schemaDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading schemas: %v\n", err)
		os.Exit(1)
	}

	output := generateERDiagram(collections)

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(output), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated ER diagram: %s\n", *outputFile)
	} else {
		fmt.Print(output)
	}
}

func loadSchemas(dir string) ([]Collection, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var collections []Collection
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}

		var schema JSONSchema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", file, err)
		}

		if schema.Schema == nil {
			continue
		}

		name := strings.TrimSuffix(filepath.Base(file), ".json")
		collection := parseCollection(name, schema.Schema)
		collections = append(collections, collection)
	}

	sort.Slice(collections, func(i, j int) bool {
		return collections[i].Name < collections[j].Name
	})

	return collections, nil
}

func parseCollection(name string, schema *SchemaDefinition) Collection {
	required := make(map[string]bool)
	for _, r := range schema.Required {
		required[r] = true
	}

	var properties []Property
	for propName, propDef := range schema.Properties {
		prop := parseProperty(propName, propDef, required[propName])
		properties = append(properties, prop)
	}

	sort.Slice(properties, func(i, j int) bool {
		// _id first, then id, then alphabetical
		if properties[i].Name == "_id" {
			return true
		}
		if properties[j].Name == "_id" {
			return false
		}
		if properties[i].Name == "id" {
			return true
		}
		if properties[j].Name == "id" {
			return false
		}
		return properties[i].Name < properties[j].Name
	})

	return Collection{
		Name:       name,
		Properties: properties,
		Required:   required,
	}
}

func parseProperty(name string, def PropertyDefinition, isRequired bool) Property {
	prop := Property{
		Name:        name,
		Type:        getBsonType(def.BsonType),
		Description: def.Description,
		IsRequired:  isRequired,
	}

	// Detect primary key
	if name == "_id" {
		prop.IsPK = true
	}

	// Detect foreign keys from description
	desc := strings.ToLower(def.Description)
	if strings.Contains(desc, "workspace id") && name != "id" {
		prop.IsFK = true
		prop.FKRef = "workspace.id"
	} else if strings.Contains(desc, "user id") && name != "id" {
		prop.IsFK = true
		prop.FKRef = "user.id"
	} else if strings.Contains(desc, "role id") && name != "id" {
		prop.IsFK = true
		prop.FKRef = "role.id"
	}

	// Handle array types
	if prop.Type == "array" && def.Items != nil {
		itemType := getBsonType(def.Items.BsonType)
		prop.Type = fmt.Sprintf("%s[]", itemType)
	}

	return prop
}

func getBsonType(t interface{}) string {
	switch v := t.(type) {
	case string:
		return v
	case []interface{}:
		// For nullable types like ["object", "null"], return the non-null type
		for _, item := range v {
			if s, ok := item.(string); ok && s != "null" {
				return s
			}
		}
		// If only null, return "null"
		return "null"
	default:
		return "unknown"
	}
}

func generateERDiagram(collections []Collection) string {
	var sb strings.Builder

	sb.WriteString("# Entity Relationship Diagram\n\n")
	sb.WriteString("<!-- This file is auto-generated by tools/cmd/ergen. Do not edit manually. -->\n\n")
	sb.WriteString("```mermaid\n")
	sb.WriteString("erDiagram\n")

	// Generate entities
	for _, coll := range collections {
		sb.WriteString(fmt.Sprintf("    %s {\n", capitalize(coll.Name)))
		for _, prop := range coll.Properties {
			constraint := ""
			if prop.IsPK {
				constraint = "PK"
			} else if prop.Name == "id" {
				constraint = "UK"
			} else if prop.IsFK {
				constraint = "FK"
			}

			comment := ""
			if prop.FKRef != "" {
				comment = fmt.Sprintf("%q", prop.FKRef)
			} else if !prop.IsRequired && prop.Name != "_id" {
				comment = "\"optional\""
			}

			if constraint != "" && comment != "" {
				sb.WriteString(fmt.Sprintf("        %s %s %s %s\n", prop.Type, prop.Name, constraint, comment))
			} else if constraint != "" {
				sb.WriteString(fmt.Sprintf("        %s %s %s\n", prop.Type, prop.Name, constraint))
			} else if comment != "" {
				sb.WriteString(fmt.Sprintf("        %s %s %s\n", prop.Type, prop.Name, comment))
			} else {
				sb.WriteString(fmt.Sprintf("        %s %s\n", prop.Type, prop.Name))
			}
		}
		sb.WriteString("    }\n\n")
	}

	// Generate relationships
	sb.WriteString("    User ||--o| Workspace : \"has personal workspace\"\n")
	sb.WriteString("    Workspace ||--o{ User : \"has members\"\n")
	sb.WriteString("    Permittable }o--|| User : \"belongs to\"\n")
	sb.WriteString("    Permittable }o--o{ Role : \"has roles\"\n")

	sb.WriteString("```\n")

	// Add relationships table
	sb.WriteString("\n## Relationships\n\n")
	sb.WriteString("| From | To | Type | Description |\n")
	sb.WriteString("|------|-----|------|-------------|\n")
	sb.WriteString("| User | Workspace | 1:1 | Each user has a personal workspace (`user.workspace` → `workspace.id`) |\n")
	sb.WriteString("| Workspace | User | 1:N | Workspace has multiple members (`workspace.members[userId]`) |\n")
	sb.WriteString("| Permittable | User | N:1 | Permittable belongs to a user (`permittable.userid` → `user.id`) |\n")
	sb.WriteString("| Permittable | Role | N:M | Permittable has multiple roles (`permittable.roleids[]` → `role.id`) |\n")

	return sb.String()
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
