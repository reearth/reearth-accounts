package main

import (
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
)

// RegisterTypes registers all document types for schema generation.
// This file is project-specific and should be modified when adding new collections.
func RegisterTypes(g *Generator) {
	g.RegisterType("UserDocument", mongodoc.UserDocument{})
	g.RegisterType("WorkspaceDocument", mongodoc.WorkspaceDocument{})
	g.RegisterType("RoleDocument", mongodoc.RoleDocument{})
	g.RegisterType("PermittableDocument", mongodoc.PermittableDocument{})
	g.RegisterType("ConfigDocument", mongodoc.ConfigDocument{})
}
