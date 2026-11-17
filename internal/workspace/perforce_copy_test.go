package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func copyRealRepository(t *testing.T) (string, string) {
	t.Helper()
	root, _ := localTestRepository(t)
	localTestWrite(t, root, "payload.txt", []byte("independent snapshot\n"))
	for _, args := range [][]string{{"add", "payload.txt"}, {"commit", "-m", "payload"}} {
		if _, err := git(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := git(context.Background(), root, "repack", "-ad"); err != nil {
		t.Fatal(err)
	}
	localTestWrite(t, root, "after-pack.txt", []byte("loose after pack\n"))
	for _, args := range [][]string{{"add", "after-pack.txt"}, {"commit", "-m", "loose"}} {
		if _, err := git(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	head, err := git(context.Background(), root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return root, head
}

func TestPerforceCopyRealObjectsRemainReadableWithoutParent(t *testing.T) {
	source, head := copyRealRepository(t)
	target := filepath.Join(t.TempDir(), "copy")
	if err := cloneLocalObjectStore(context.Background(), source, target, localManifest{}); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	before, err := inspectLocalObjectStore(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.files) == 0 {
		t.Fatal("fixture did not contain actual objects")
	}
	for name, original := range before.files {
		copied, err := os.Lstat(filepath.Join(target, ".git", "objects", name))
		if err != nil || os.SameFile(original, copied) {
			t.Fatalf("object was not independently copied %s: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(target, "payload.txt")); !os.IsNotExist(err) {
		t.Fatalf("clone checked out worktree: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(target, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatalf("clone borrowed objects: %v", err)
	}
	offline := filepath.Join(t.TempDir(), "offline-parent")
	if err := os.Rename(source, offline); err != nil {
		t.Fatal(err)
	}
	body, err := gitBytes(context.Background(), target, nil, "show", head+":payload.txt")
	if err != nil || string(body) != "independent snapshot\n" {
		t.Fatalf("clone depends on missing parent: %q %v", body, err)
	}
	if _, err := git(context.Background(), target, "fsck", "--no-dangling"); err != nil {
		t.Fatal(err)
	}
}
