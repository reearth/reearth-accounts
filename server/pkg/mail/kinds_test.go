package mail

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindsListsEveryDeclaredKind(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	declared := map[string]string{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					id, ok := vs.Type.(*ast.Ident)
					if !ok || id.Name != "Kind" {
						continue
					}
					for i, name := range vs.Names {
						lit, ok := vs.Values[i].(*ast.BasicLit)
						require.True(t, ok, "Kind constant %s must be a string literal", name.Name)
						v, err := strconv.Unquote(lit.Value)
						require.NoError(t, err)
						declared[v] = name.Name
					}
				}
			}
		}
	}
	require.NotEmpty(t, declared)

	listed := map[string]bool{}
	for _, k := range Kinds {
		listed[string(k)] = true
		assert.Contains(t, declared, string(k), "Kinds entry %q is not a declared Kind constant", k)
	}
	for v, name := range declared {
		assert.True(t, listed[v], "Kind constant %s is missing from Kinds", name)
	}
}
