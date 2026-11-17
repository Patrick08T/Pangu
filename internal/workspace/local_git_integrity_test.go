package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func localTestMetadataTree(t *testing.T, repository, parent string, data []byte) string {
	t.Helper()
	return localTestMetadataFile(t, repository, parent, ".gitmodules", int64(len(data)), bytes.NewReader(data))
}

func localTestMetadataFile(t *testing.T, repository, parent, name string, size int64, data io.Reader) string {
	t.Helper()
	header := fmt.Sprintf("commit refs/heads/metadata\ncommitter Test <test@localhost> 0 +0000\ndata 1\nx\nfrom %s\nM 100644 inline %s\ndata %d\n", parent, name, size)
	cmd := localGitCommand(context.Background(), repository, "fast-import", "--quiet", "--done")
	cmd.Stdin = io.MultiReader(strings.NewReader(header), data, strings.NewReader("\n\ndone\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("metadata fixture: %v %s", err, stderr.String())
	}
	commit, err := git(context.Background(), repository, "rev-parse", "metadata")
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func TestLocalGitRawMetadataFsckContract(t *testing.T) {
	cases := map[string]string{
		"parse":  "[not closed\n\x00\xffraw\r\n",
		"name":   "[submodule \"../unsafe\"]\npath = ordinary\nurl = local\n",
		"path":   "[submodule \"ordinary\"]\npath = -unsafe\nurl = local\n",
		"url":    "[submodule \"ordinary\"]\npath = ordinary\nurl = -unsafe\n",
		"update": "[submodule \"ordinary\"]\npath = ordinary\nurl = local\nupdate = !unsafe\n",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			repository, parent := localTestRepository(t)
			commit := localTestMetadataTree(t, repository, parent, []byte(data))
			if _, err := git(context.Background(), repository, localObjectCheckArgs(commit)...); err != nil {
				t.Fatalf("ordinary P4 metadata bytes rejected: %v", err)
			}
		})
	}
}

func TestLocalGitObjectCheckStillRejectsMissingAndCorruptBlobs(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			repository, parent := localTestRepository(t)
			commit := localTestMetadataTree(t, repository, parent, []byte("[submodule \"ordinary\"]\npath = data\nurl = local\n"))
			oid, err := git(context.Background(), repository, "rev-parse", commit+":.gitmodules")
			if err != nil || !commitID.MatchString(oid) {
				t.Fatalf("blob identity: %q %v", oid, err)
			}
			objects := filepath.Join(repository, ".git", "objects")
			target := filepath.Join(objects, oid[:2], oid[2:])
			rel, err := filepath.Rel(objects, target)
			if err != nil || !filepath.IsLocal(rel) || !filepath.IsAbs(target) {
				t.Fatalf("fixture target escaped repository: %q %v", target, err)
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatal(err)
			}
			if mode == "missing" {
				err = os.Remove(target)
			} else {
				// Git marks its own loose objects readonly; clear only this fixture attribute.
				if err = os.Chmod(target, 0600); err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(target, []byte("not a zlib object"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := git(context.Background(), repository, localObjectCheckArgs(commit)...); err == nil {
				t.Fatal("damaged object accepted")
			}
		})
	}
}

func TestLocalGitImportPreservesInvalidGitmodulesRawBytes(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	data := []byte("[not closed\n\x00\xffraw\r\n")
	localTestWrite(t, root, ".gitmodules", data)
	m, err := inspectLocalFiles(context.Background(), root, []string{".gitmodules"})
	if err != nil {
		t.Fatal(err)
	}
	commit, err := importLocalFiles(context.Background(), repository, root, parent, m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := gitBytes(context.Background(), repository, nil, "show", commit+":.gitmodules")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("raw gitmodules changed: %q %v", got, err)
	}
	if strings.TrimSpace(commit) == "" {
		t.Fatal("no validated commit")
	}
}
