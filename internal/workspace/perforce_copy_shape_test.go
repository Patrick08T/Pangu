package workspace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestPerforceCopyFunctionBounds(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "perforce_copy") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				lines := set.Position(node.End()).Line - set.Position(node.Pos()).Line + 1
				if lines > 70 {
					t.Errorf("%s function exceeds 70 lines: %d", set.Position(node.Pos()), lines)
				}
			}
			return true
		})
	}
}
