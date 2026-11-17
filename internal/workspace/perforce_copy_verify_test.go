package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPerforceCopyVerificationRejectsChangedSourceAndHardlinks(t *testing.T) {
	for _, change := range []string{"source", "hardlink", "missing", "metadata"} {
		t.Run(change, func(t *testing.T) {
			source := copyTestStore(t)
			before, err := inspectLocalObjectStore(context.Background(), source)
			if err != nil {
				t.Fatal(err)
			}
			target := t.TempDir()
			for name := range before.files {
				body, err := os.ReadFile(filepath.Join(source, ".git", "objects", name))
				if err != nil {
					t.Fatal(err)
				}
				if change == "missing" && name == "aa/bb" {
					continue
				}
				if change == "hardlink" && name == "aa/bb" {
					file := filepath.Join(target, ".git", "objects", name)
					if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Link(filepath.Join(source, ".git", "objects", name), file); err != nil {
						t.Fatal(err)
					}
					continue
				}
				localTestWrite(t, target, ".git/objects/"+name, body)
			}
			if change == "source" {
				localTestWrite(t, source, ".git/objects/aa/bb", []byte("changed size"))
			}
			if change == "metadata" {
				localTestWrite(t, source, ".git/config", []byte("[user]\n name = changed\n"))
			}
			if err := verifyLocalObjectCopy(context.Background(), source, target, before); err == nil {
				t.Fatal("invalid clone was publishable", change)
			}
		})
	}
}

func TestPerforceCopyInsufficientHistoryCapacityStartsNoClone(t *testing.T) {
	source := copyTestStore(t)
	target := filepath.Join(t.TempDir(), "copy")
	// Current manifest is empty. Old policy admits exactly R; parent objects still cost bytes.
	probe := func(string) (uint64, uint64, error) { return 10 << 30, 100 << 30, nil }
	err := cloneLocalObjectStoreWithSpace(context.Background(), source, target, localManifest{}, probe)
	if err == nil || !strings.Contains(err.Error(), "空间不足") {
		t.Fatalf("historical objects omitted from admission: %v", err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("clone started before history admission: %v", err)
	}
	store := localObjectStore{bytes: 1}
	probe = func(string) (uint64, uint64, error) { return (10 << 30) + 1, 100 << 30, nil }
	if err := checkLocalObjectCapacity(context.Background(), filepath.Dir(target), store, localManifest{}, probe); err != nil {
		t.Fatalf("exact boundary refused: %v", err)
	}
}
