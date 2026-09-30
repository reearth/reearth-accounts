package migration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every collection name passed to ApplyCollectionSchemas must resolve to an
// embedded schema file (<name>.json). A name that does not resolve - such as the
// plural "workspaces" instead of "workspace" - makes the migration fail at
// runtime with "failed to read schema file", which leaves the collection
// validator stale while newly deployed code writes fields the old validator
// rejects. Nothing else in the build catches that, so check the call sites here.
func TestApplyCollectionSchemasUsesEmbeddedSchemaNames(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	names := 0
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				fn, ok := call.Fun.(*ast.Ident)
				if !ok || fn.Name != "ApplyCollectionSchemas" || len(call.Args) < 2 {
					return true
				}
				lit, ok := call.Args[1].(*ast.CompositeLit)
				if !ok {
					return true
				}
				for _, elt := range lit.Elts {
					bl, ok := elt.(*ast.BasicLit)
					if !ok || bl.Kind != token.STRING {
						continue
					}
					name, err := strconv.Unquote(bl.Value)
					require.NoError(t, err)
					names++
					_, err = schema.SchemaFS.ReadFile(name + ".json")
					assert.NoErrorf(t, err, "%s: collection %q has no embedded schema file %s.json",
						filepath.Base(path), name, name)
				}
				return true
			})
		}
	}

	assert.NotZero(t, names, "no ApplyCollectionSchemas call sites found")
}
