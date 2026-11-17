package learning

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestRetentionFunctionsRemainBounded(t *testing.T) {
	paths, err := filepath.Glob("retention*.go")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, "source.go")
	for _, path := range paths {
		files := token.NewFileSet()
		file, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			lines := files.Position(function.End()).Line - files.Position(function.Pos()).Line + 1
			if lines > 70 {
				t.Errorf("%s %s has %d lines; maximum 70", path, function.Name.Name, lines)
			}
		}
	}
}
