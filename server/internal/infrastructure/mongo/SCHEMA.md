# MongoDB Schema Management

This document explains how to add or update MongoDB collection schemas.

See also: [ER Diagram](schema/ER.md)

## Directory Structure

```
infrastructure/mongo/
├── schema/
│   ├── schema.go          # Embeds JSON schema files
│   ├── user.json          # User collection schema
│   ├── workspace.json     # Workspace collection schema
│   ├── role.json          # Role collection schema
│   ├── permittable.json   # Permittable collection schema
│   └── config.json        # Config collection schema
└── migration/
    ├── migrations.go      # Migration registry
    └── apply_collection_schemas.go  # Schema application logic
```

## Adding a New Collection Schema

1. **Create the schema file** `schema/<collection_name>.json`:

```json
{
  "$jsonSchema": {
    "bsonType": "object",
    "title": "<CollectionName> Collection Schema",
    "description": "Schema for <collection_name> documents",
    "required": ["id", "requiredField"],
    "properties": {
      "_id": {
        "bsonType": "objectId",
        "description": "MongoDB internal ID"
      },
      "id": {
        "bsonType": "string",
        "description": "<CollectionName> ID (ULID format)"
      },
      "requiredField": {
        "bsonType": "string",
        "description": "Description of required field"
      },
      "optionalField": {
        "bsonType": "string",
        "description": "Description of optional field. Default: \"\""
      }
    },
    "additionalProperties": false
  }
}
```

2. **Create a migration file** `migration/<timestamp>_apply_<collection_name>_schema.go`:

```go
package migration

import (
    "context"
)

func Apply<CollectionName>Schema(ctx context.Context, c DBClient) error {
    return ApplyCollectionSchemas(ctx, []string{"<collection_name>"}, c)
}
```

3. **Register the migration** in `migration/migrations.go`:

```go
var migrations = migration.Migrations[DBClient]{
    // ... existing migrations
    <timestamp>: Apply<CollectionName>Schema,
}
```

## Adding a New Property to Existing Schema

1. **Update the schema file** `schema/<collection_name>.json`:
   - Add the new property to `properties`
   - If required, add the field name to `required` array
   - Include default value in description for optional fields

2. **Create a migration file** to apply the updated schema:

```go
package migration

import (
    "context"
)

func Update<CollectionName>Schema(ctx context.Context, c DBClient) error {
    return ApplyCollectionSchemas(ctx, []string{"<collection_name>"}, c)
}
```

3. **Register the migration** in `migration/migrations.go`

## Schema Validation Settings

Schemas are applied with:

- **validationLevel**: `moderate` - Validates inserts and updates on existing valid documents
- **validationAction**: `warn` - Logs validation failures but allows the operation

## additionalProperties

`additionalProperties` controls whether properties not defined in the schema are allowed in the document.

- `true` - Allows undefined properties
- `false` - Rejects undefined properties

**Policy: All schemas must set `additionalProperties: false`**

This ensures:
- Prevention of typos and invalid field names
- Strict schema enforcement
- Data consistency

```json
{
  "$jsonSchema": {
    "properties": {
      "id": { "bsonType": "string" },
      "name": { "bsonType": "string" }
    },
    "additionalProperties": false
  }
}
```

With the above setting, documents containing fields other than `id` and `name` will fail validation.

## BSON Types Reference

| Type | Description |
|------|-------------|
| string | UTF-8 string |
| int | 32-bit integer |
| long | 64-bit integer |
| bool | Boolean |
| date | UTC datetime |
| objectId | MongoDB ObjectId |
| array | Array |
| object | Embedded document |
| binData | Binary data |
