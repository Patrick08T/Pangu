package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalGitRejectsUnsafeOrAmbiguousNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", "a/../b", "a//b", "./file", ".git/x", "a/.GiT/x", "a/.git.", "file:stream", "con.txt", "x\\y", "bad\x00name", "a/.../b", "a/\u200cb"} {
		t.Run(strings.ReplaceAll(name, "\x00", "NUL"), func(t *testing.T) {
			if _, err := inspectLocalFiles(context.Background(), root, []string{name}); err == nil {
				t.Fatalf("unsafe path accepted: %q", name)
			}
		})
	}
	localTestWrite(t, root, "file", []byte("x"))
	if _, err := inspectLocalFiles(context.Background(), root, []string{"file", "FILE"}); err == nil {
		t.Fatal("case aliases accepted")
	}
	localTestWrite(t, root, ".git", []byte("gitdir: elsewhere"))
	if _, err := inspectLocalFiles(context.Background(), root, []string{"file"}); err == nil {
		t.Fatal("root metadata ignored")
	}
}

func TestLocalGitRejectsJunctionEvenWhenDescendantMissing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction check")
	}
	root, outside := t.TempDir(), t.TempDir()
	alias := filepath.Join(root, "alias")
	if err := exec.Command("cmd", "/c", "mklink", "/J", alias, outside).Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectLocalFiles(context.Background(), root, []string{"alias/missing"}); err == nil {
		t.Fatal("junction missing descendant treated as clean")
	}
}

func TestLocalGitRejectsStaleIdentityAndInvalidParentWithoutPublishing(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	localTestWrite(t, root, "file", []byte("first"))
	m, err := inspectLocalFiles(context.Background(), root, []string{"file"})
	if err != nil {
		t.Fatal(err)
	}
	// Rename the original aside: even equal size/time must not bless another file identity.
	if err := os.Rename(filepath.Join(root, "file"), filepath.Join(root, "old")); err != nil {
		t.Fatal(err)
	}
	localTestWrite(t, root, "file", []byte("other"))
	if err := os.Chtimes(filepath.Join(root, "file"), m.Files[0].ModTime, m.Files[0].ModTime); err != nil {
		t.Fatal(err)
	}
	if _, err := importLocalFiles(context.Background(), repository, root, parent, m); err == nil {
		t.Fatal("replacement identity accepted")
	}
	for _, invalid := range []string{"", "HEAD", strings.Repeat("0", 40)} {
		if _, err := importLocalFiles(context.Background(), repository, root, invalid, m); err == nil {
			t.Fatalf("invalid parent: %q", invalid)
		}
	}
	assertLocalNoPublishedImport(t, repository, parent)
}

func assertLocalNoPublishedImport(t *testing.T, repository, parent string) {
	t.Helper()
	head, err := git(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil || head != parent {
		t.Fatalf("HEAD changed: %q %v", head, err)
	}
	refs, err := git(context.Background(), repository, "for-each-ref", "--format=%(refname)")
	if err != nil || strings.Contains(refs, "refs/heads/local-") || strings.Contains(refs, "refs/pangu-import/") {
		t.Fatalf("partial refs published: %q %v", refs, err)
	}
}

func TestLocalGitHonorsParentCancellation(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	m, err := inspectLocalFiles(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, inspectErr := inspectLocalFiles(ctx, root, nil)
	_, importErr := importLocalFiles(ctx, repository, root, parent, m)
	_, commandErr := runLocalGit(ctx, repository, "rev-parse", "HEAD")
	streamErr := streamLocalGit(ctx, repository, []string{"ls-tree", "-rz", parent}, func([]byte) error { return nil })
	for _, err := range []error{inspectErr, importErr, commandErr, streamErr} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	}
	assertLocalNoPublishedImport(t, repository, parent)
}

func TestLocalGitQuotesUnicodeSeparatorsAsRawPathBytes(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	name := "hello\u00a0world"
	localTestWrite(t, root, name, []byte("unchanged"))
	m, err := inspectLocalFiles(context.Background(), root, []string{name})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := importLocalFiles(context.Background(), repository, root, parent, m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := git(context.Background(), repository, "show", commit+":"+name)
	if err != nil || got != "unchanged" {
		t.Fatalf("path bytes changed: %q %v", got, err)
	}
}
