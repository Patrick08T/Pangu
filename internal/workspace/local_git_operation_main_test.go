package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Standalone helper tests need a private managed root; explicit fixture roots win.
func TestMain(m *testing.M) {
	if os.Getenv("PANGU_WORKSPACE_ROOT") != "" {
		os.Exit(m.Run())
	}
	root, err := os.MkdirTemp("", "pangu-operation-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.Setenv("PANGU_WORKSPACE_ROOT", root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := m.Run()
	if !filepath.IsAbs(root) || filepath.Dir(root) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(root), "pangu-operation-tests-") {
		fmt.Fprintln(os.Stderr, "refusing to remove unexpected test root")
		os.Exit(2)
	}
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(code)
}
