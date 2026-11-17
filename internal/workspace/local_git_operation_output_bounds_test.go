package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func localOperationStoreBlob(t *testing.T, repository string, body []byte) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, "bounded.bin"), body, 0600); err != nil {
		t.Fatal(err)
	}
	object, err := git(context.Background(), repository, "hash-object", "-w", "--", "bounded.bin")
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func TestLocalGitOperationLocalBlobPreservesRawBytesAndTenMiBBound(t *testing.T) {
	repository, _, object, body := localOperationOutputFixture(t)
	set := &Fileset{repository: repository, localPerforce: true}
	if data, err := set.Blob(context.Background(), object); err != nil || !bytes.Equal(data, body) {
		t.Fatalf("local Blob changed raw bytes: %q %v", data, err)
	}
	body = bytes.Repeat([]byte{0, 255}, 5*1024*1024)
	object = localOperationStoreBlob(t, repository, body)
	if data, err := set.Blob(context.Background(), object); err != nil || !bytes.Equal(data, body) {
		t.Fatalf("exact 10MiB Blob rejected/changed: length=%d error=%v", len(data), err)
	}
	object = localOperationStoreBlob(t, repository, append(body, 0))
	if data, err := set.Blob(context.Background(), object); err == nil || data != nil {
		t.Fatalf("oversized Blob returned data: length=%d error=%v", len(data), err)
	}
}

func TestLocalGitOperationArtifactQueryKeepsSixteenMiBErrorContract(t *testing.T) {
	repository, _, _, _ := localOperationOutputFixture(t)
	set := &Fileset{repository: repository, localPerforce: true}
	body := bytes.Repeat([]byte{0, 255}, 8*1024*1024)
	object := localOperationStoreBlob(t, repository, body)
	data, err := set.bytes(context.Background(), "cat-file", "blob", object)
	if err != nil || !bytes.Equal(data, body) {
		t.Fatalf("exact 16MiB query rejected/changed: length=%d error=%v", len(data), err)
	}
	object = localOperationStoreBlob(t, repository, append(body, 0))
	data, err = set.bytes(context.Background(), "cat-file", "blob", object)
	if err == nil || len(data) != 16*1024*1024 || !bytes.Equal(data, body) {
		t.Fatalf("overflow query lost bounded-prefix/error contract: length=%d error=%v", len(data), err)
	}
}

func TestLocalGitOperationArtifactDiffKeepsOneMiBTruncation(t *testing.T) {
	repository, base, _, _ := localOperationOutputFixture(t)
	body := bytes.Repeat([]byte("visible text line\n"), 100_000)
	if err := os.WriteFile(filepath.Join(repository, "large.txt"), body, 0600); err != nil {
		t.Fatal(err)
	}
	largeRecoveryGit(t, repository, "add", "--", "large.txt")
	largeRecoveryGit(t, repository, "commit", "-m", "bounded diff")
	head := largeRecoveryGit(t, repository, "rev-parse", "HEAD")
	set := &Fileset{repository: repository, Base: base, Head: head, localPerforce: true}
	data, cut, err := set.Diff(context.Background(), "large.txt")
	if err != nil || !cut || len(data) != 1024*1024 || !bytes.Contains(data, []byte("+visible text line\n")) {
		t.Fatalf("local diff lost bounded successful prefix: length=%d cut=%v error=%v", len(data), cut, err)
	}
}
