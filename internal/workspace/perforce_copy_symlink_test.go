package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestPerforceCopyRejectsFileSymlinksBeforeGit(t *testing.T) {
	for _, name := range []string{"config", "HEAD", "refs/heads/main", "objects/aa/link"} {
		t.Run(name, func(t *testing.T) {
			root, outside := copyTestStore(t), t.TempDir()
			localTestWrite(t, outside, "sentinel", []byte("untouched"))
			link := filepath.Join(root, ".git", filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			err := os.Symlink(filepath.Join(outside, "sentinel"), link)
			if runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(1314)) || errors.Is(err, os.ErrPermission)) {
				t.Skip("OS file-symlink privilege unavailable; separate real junction test is mandatory")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := inspectLocalObjectStore(context.Background(), root); err == nil {
				t.Fatal("source metadata file link accepted")
			}
			body, err := os.ReadFile(filepath.Join(outside, "sentinel"))
			if err != nil || string(body) != "untouched" {
				t.Fatalf("outside target changed: %q %v", body, err)
			}
		})
	}
}
