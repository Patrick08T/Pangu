package workspace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestLocalImportGuardFunctionBounds(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "local_git_import.go" && name != "local_git_cleanup.go" && !(strings.HasPrefix(name, "local_git_import_guard") && strings.HasSuffix(name, ".go")) {
			continue
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, name, nil, 0)
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
