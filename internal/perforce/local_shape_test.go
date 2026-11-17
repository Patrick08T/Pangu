package perforce

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestLocalPerforceFunctionBounds(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("discover sources: %v %v", files, err)
	}
	for _, path := range files {
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			var body *ast.BlockStmt
			switch n := node.(type) {
			case *ast.FuncDecl:
				body = n.Body
			case *ast.FuncLit:
				body = n.Body
			}
			if body != nil {
				lines := set.Position(body.End()).Line - set.Position(node.Pos()).Line + 1
				if lines > 70 {
					t.Errorf("%s:%d function exceeds 70 lines: %d", path, set.Position(node.Pos()).Line, lines)
				}
			}
			return true
		})
	}
}
