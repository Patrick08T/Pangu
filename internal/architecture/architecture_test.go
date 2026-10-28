package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestModuleBoundariesAndFunctionSize(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(current)))
	allowed := map[string][]string{
		"workflow": {}, "database": {}, "diagnostic": {}, "agent": {"workflow", "diagnostic", "catalog", "research"},
		"research":   {"workflow", "workspace", "database", "diagnostic"},
		"identity":   {"database", "audit"},
		"audit":      {"database"},
		"execution":  {"database", "diagnostic", "workflow", "agent"},
		"workspace":  {"database", "diagnostic", "workflow"},
		"artifact":   {"workspace", "diagnostic", "research"},
		"catalog":    {"database", "workflow"},
		"submission": {"database", "workflow", "diagnostic", "identity"},
		"schedule":   {"database", "workflow", "submission", "identity"},
		"scheduler":  {"workflow", "diagnostic", "execution", "workspace", "submission", "schedule", "catalog", "research", "agent"}, "api": {"workflow", "scheduler", "database", "execution", "workspace", "artifact", "submission", "schedule", "catalog", "identity", "audit"},
	}
	for _, directory := range []string{"cmd", "internal", "api", "web"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, err error) error {
			if err == nil && directory == "web" && entry.IsDir() && path != filepath.Join(root, "web") {
				return filepath.SkipDir
			}
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
				return err
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				return err
			}
			checkFunctions(t, path, positions, file)
			if directory == "internal" && !strings.HasSuffix(path, "_test.go") {
				checkImports(t, filepath.Base(filepath.Dir(path)), allowed, file)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkFunctions(t *testing.T, path string, positions *token.FileSet, file *ast.File) {
	t.Helper()
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Body != nil {
			lines := positions.Position(function.End()).Line - positions.Position(function.Pos()).Line + 1
			if lines > 70 {
				t.Errorf("%s: %s 超过 70 行（%d）", path, function.Name, lines)
			}
		}
	}
}

func checkImports(t *testing.T, module string, allowed map[string][]string, file *ast.File) {
	t.Helper()
	for _, item := range file.Imports {
		path, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(path, "github.com/Patrick08T/Pangu/internal/") {
			continue
		}
		dependency := strings.TrimPrefix(path, "github.com/Patrick08T/Pangu/internal/")
		found := false
		for _, permitted := range allowed[module] {
			found = found || dependency == permitted
		}
		if !found {
			t.Errorf("模块 %s 不允许依赖 %s；修改边界前先更新架构设计", module, dependency)
		}
	}
}
