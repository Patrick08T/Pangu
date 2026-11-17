package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func localTestRepository(t *testing.T) (string, string) {
	t.Helper()
	repository := t.TempDir()
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"commit", "--allow-empty", "-m", "root"}} {
		if _, err := git(context.Background(), repository, args...); err != nil {
			t.Fatal(err)
		}
	}
	parent, err := git(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return repository, parent
}

func localTestWrite(t *testing.T, root, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalGitManifestCountsRawFilesAndMissing(t *testing.T) {
	root := t.TempDir()
	localTestWrite(t, root, "nested/普通.txt", []byte("a\r\nb"))
	localTestWrite(t, root, "zero", nil)
	m, err := inspectLocalFiles(context.Background(), root, []string{"zero", "absent", "nested/普通.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 2 || m.Missing != 1 || m.TotalBytes != 4 || m.Largest != 4 || m.PathBytes != 21 {
		t.Fatalf("manifest counts: %+v", m)
	}
}

func TestLocalGitImportPreservesRawBytesWithoutFiltersOrGitlink(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	files := map[string][]byte{
		".gitattributes":      []byte("* text eol=lf filter=lfs\n"),
		"nested/raw.bin":      {0, 255, 13, 10, 10, 13, 'x'},
		"nested/普通 space.txt": []byte("a\r\nb\n"),
	}
	localTestWrite(t, root, "nested/.git/HEAD", []byte("ref: refs/heads/main\n"))
	var names []string
	for name, data := range files {
		localTestWrite(t, root, name, data)
		names = append(names, name)
	}
	m, err := inspectLocalFiles(context.Background(), root, names)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := importLocalFiles(context.Background(), repository, root, parent, m)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := gitBytes(context.Background(), repository, nil, "show", commit+":"+name)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("raw bytes %s: %q %v", name, got, err)
		}
	}
	if err := inspectLocalRepository(context.Background(), repository, commit); err != nil {
		t.Fatal(err)
	}
	head, _ := git(context.Background(), repository, "rev-parse", "HEAD")
	refs, _ := git(context.Background(), repository, "for-each-ref", "--format=%(refname)")
	if head != parent || !strings.Contains(refs, "refs/heads/local-") || strings.Contains(refs, "refs/pangu-import/") {
		t.Fatalf("published wrong refs/head: %q %q", head, refs)
	}
}

func TestLocalGitRehashesSameSizeMtimeAndDeletion(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	localTestWrite(t, root, "file", []byte("first"))
	m, err := inspectLocalFiles(context.Background(), root, []string{"file"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := importLocalFiles(context.Background(), repository, root, parent, m)
	if err != nil {
		t.Fatal(err)
	}
	localTestWrite(t, root, "file", []byte("other"))
	if err := os.Chtimes(filepath.Join(root, "file"), m.Files[0].ModTime, m.Files[0].ModTime); err != nil {
		t.Fatal(err)
	}
	second, err := importLocalFiles(context.Background(), repository, root, first, m)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := git(context.Background(), repository, "show", second+":file")
	if got != "other" {
		t.Fatalf("mtime cache hid content: %q", got)
	}
	if err := os.Remove(filepath.Join(root, "file")); err != nil {
		t.Fatal(err)
	}
	missing, err := inspectLocalFiles(context.Background(), root, []string{"file"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := importLocalFiles(context.Background(), repository, root, second, missing)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = git(context.Background(), repository, "ls-tree", "-r", third)
	if got != "" {
		t.Fatalf("delete did not remove parent tree: %q", got)
	}
}

func TestLocalGitStreamsNamesAndRunsBulkCommand(t *testing.T) {
	repository, parent := localTestRepository(t)
	out, err := runLocalGit(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil || out != parent {
		t.Fatalf("bulk: %q %v", out, err)
	}
	var records []string
	err = streamLocalGit(context.Background(), repository, []string{"log", "-z", "--format=%H", "-1"}, func(p []byte) error {
		records = append(records, string(p))
		return nil
	})
	if err != nil || len(records) != 1 || records[0] != parent {
		t.Fatalf("stream: %q %v", records, err)
	}
	free, total, err := localVolumeSpace(repository)
	if err != nil || free == 0 || total < free {
		t.Fatalf("space: %d/%d %v", free, total, err)
	}
}
