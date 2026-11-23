package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Request handlers reach sessions only through the guard. The store has two
// ways out of it, and each is allowed in exactly one place: Unchecked in
// Server.Sessions (for embedding programs) and the configured store field
// in New and its option (wiring the guard and the usage observer).
func TestOnlySessionsReachesTheStoreUnchecked(t *testing.T) {
	allowed := map[string]map[string]bool{
		"Unchecked": {"Sessions": true},
		"storeOpt":  {"New": true, "WithSessionStore": true},
	}
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if where, guarded := allowed[sel.Sel.Name]; guarded && !where[fn.Name.Name] {
					t.Errorf("%s: %s uses .%s; handlers reach sessions through s.guard.For(p)", fset.Position(sel.Pos()), fn.Name.Name, sel.Sel.Name)
				}
				return true
			})
		}
	}
}
