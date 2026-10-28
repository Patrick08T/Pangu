package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectRejectsUnsafeVerificationCommand(t *testing.T) {
	binary, _ := os.Executable()
	for _, command := range [][]string{{"go", "test"}, {binary, "token=hidden-value"}, {binary, string([]byte{0})}} {
		project := Project{Name: "验证", Source: t.TempDir(), Branch: "main", WorkerID: "test", Verification: command}
		if project.Validate() == nil {
			t.Fatalf("接受无效命令: %q", command)
		}
	}
	p := Project{Name: "验证", Source: t.TempDir(), Branch: "main", WorkerID: "test", Verification: []string{filepath.Clean(binary), "verify-helper", "pass"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
