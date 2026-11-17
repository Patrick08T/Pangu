package scheduler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestExperienceErasureFunctionBounds(t *testing.T) {
	for _, pattern := range []string{"experience_erasure*.go", "../workspace/experience_erasure*.go"} {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range file.Decls {
				function, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				size := positions.Position(function.End()).Line - positions.Position(function.Pos()).Line + 1
				if size > 70 {
					t.Errorf("%s:%s has %d lines, maximum 70", path, function.Name.Name, size)
				}
			}
		}
	}
}
