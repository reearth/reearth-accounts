package main

import (
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
)

// RegisterSchemas registers all collection schemas for schema generation.
// This file is project-specific and should be modified when adding new collections.
func RegisterSchemas(g *Generator) {
	g.RegisterSchema(
		"user",
		mongodoc.UserDocument{},
		"User Collection Schema",
		"Schema for user documents in the reearth-accounts database",
	)
	g.RegisterSchema(
		"workspace",
		mongodoc.WorkspaceDocument{},
		"Workspace Collection Schema",
		"Schema for workspace documents in the reearth-accounts database",
	)
	g.RegisterSchema(
		"role",
		mongodoc.RoleDocument{},
		"Role Collection Schema",
		"Schema for role documents in the reearth-accounts database",
	)
	g.RegisterSchema(
		"permittable",
		mongodoc.PermittableDocument{},
		"Permittable Collection Schema",
		"Schema for permittable documents in the reearth-accounts database",
	)
	g.RegisterSchema(
		"config",
		mongodoc.ConfigDocument{},
		"Config Collection Schema",
		"Schema for config documents in the reearth-accounts database",
	)
}
