package experience

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestFunctionsStayWithinSeventyLines(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		positions := token.NewFileSet()
		parsed, err := parser.ParseFile(positions, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			start, end := positions.Position(function.Pos()).Line, positions.Position(function.End()).Line
			if end-start+1 > 70 {
				t.Errorf("%s:%d %s has %d lines; maximum 70", entry.Name(), start, function.Name, end-start+1)
			}
		}
	}
}
