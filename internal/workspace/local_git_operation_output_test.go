package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func localOperationOutputFixture(t *testing.T) (string, string, string, []byte) {
	t.Helper()
	localOperationFixture(t)
	repository, commit := localTestRepository(t)
	body := []byte(" \x00\xff\r\nbody \t\n")
	if err := os.WriteFile(filepath.Join(repository, "raw.bin"), body, 0600); err != nil {
		t.Fatal(err)
	}
	object, err := git(context.Background(), repository, "hash-object", "-w", "--", "raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	return repository, commit, object, body
}

func TestLocalGitOperationOutputPreservesRawBytesAndTruncation(t *testing.T) {
	repository, _, object, body := localOperationOutputFixture(t)
	for _, limit := range []int{len(body), 4} {
		data, cut, err := runLocalGitOutput(context.Background(), repository, limit, "cat-file", "blob", object)
		if err != nil || !bytes.Equal(data, body[:limit]) || cut != (limit < len(body)) {
			t.Fatalf("raw output changed: limit=%d got=%q cut=%v error=%v", limit, data, cut, err)
		}
	}
	if err := checkLocalGitOperation(context.Background()); err != nil {
		t.Fatalf("confirmed output command retained quarantine: %v", err)
	}
}

func TestLocalGitOperationOutputErrorDoesNotReturnPartialSuccess(t *testing.T) {
	repository, _, _, _ := localOperationOutputFixture(t)
	data, cut, err := runLocalGitOutput(context.Background(), repository, 4, "cat-file", "blob", strings.Repeat("f", 40))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || data != nil || cut || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("ordinary exit changed output/error contract: %q %v %v", data, cut, err)
	}
	if err := checkLocalGitOperation(context.Background()); err != nil {
		t.Fatalf("ordinary exit retained quarantine: %v", err)
	}
}

func TestLocalGitOperationOutputRejectsQuarantine(t *testing.T) {
	repository, _, object, _ := localOperationOutputFixture(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(ctx, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	data, cut, err := runLocalGitOutput(context.Background(), repository, 4, "cat-file", "blob", object)
	if !errors.Is(err, errLocalGitOperationBlocked) || data != nil || cut {
		t.Fatalf("byte-output command bypassed quarantine: %q %v %v", data, cut, err)
	}
}

func TestLocalGitOperationGenericFilesetIgnoresLocalQuarantine(t *testing.T) {
	repository, commit, object, body := localOperationOutputFixture(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(ctx, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	set := &Fileset{repository: repository, Base: commit, Head: commit}
	data, err := set.Blob(context.Background(), object)
	if err != nil || !bytes.Equal(data, body) {
		t.Fatalf("generic Fileset inherited local quarantine or byte conversion: %q %v", data, err)
	}
	if data, cut, err := set.Diff(context.Background(), "raw.bin"); err != nil || cut || len(data) != 0 {
		t.Fatalf("generic diff inherited local quarantine: %q %v %v", data, cut, err)
	}
}
