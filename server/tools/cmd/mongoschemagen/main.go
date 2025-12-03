// mongoschemagen generates MongoDB $jsonSchema from Go struct definitions.
//
// Usage:
//
//	go run ./tools/cmd/mongoschemagen -config schemas.yml -output ./internal/infrastructure/mongo/schema
package main

func main() {
	g := NewGenerator()
	RegisterTypes(g)
	g.RunCLI()
}
