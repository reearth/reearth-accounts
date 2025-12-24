// mongoschemagen generates MongoDB $jsonSchema from Go struct definitions.
//
// Usage:
//
//	go run ./tools/cmd/mongoschemagen -output ./internal/infrastructure/mongo/schema
package main

func main() {
	g := NewGenerator()
	RegisterSchemas(g)
	g.RunCLI()
}
