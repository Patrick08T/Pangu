//go:build windows

package perforce

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalInventoryRejectsJunctionBeforeExcludedMetadata(t *testing.T) {
	for _, marker := range []bool{false, true} {
		t.Run(map[bool]string{false: "ancestor", true: "marker"}[marker], func(t *testing.T) {
			s := inventorySnapshot(t, []string{"//daily/项目/repo/.git/config"})
			target := t.TempDir()
			if err := os.WriteFile(filepath.Join(target, "config"), []byte("private"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(s.LocalRoot, "项目", "repo")
			if marker {
				if err := os.MkdirAll(link, 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(link, ".git")
			}
			cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("junction fixture failed: %s %v", output, err)
			}
			got, err := s.LocalFiles(t.Context())
			if err == nil || got != nil {
				t.Fatalf("junction hidden by exclusion: %v %v", got, err)
			}
			data, err := os.ReadFile(filepath.Join(target, "config"))
			if err != nil || string(data) != "private" {
				t.Fatalf("junction target touched: %q %v", data, err)
			}
		})
	}
}
