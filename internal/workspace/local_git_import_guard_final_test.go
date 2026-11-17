package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalImportGuardFinalReserveCheckedAfterSuccessfulImport(t *testing.T) {
	localOperationFixture(t)
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	manifest, err := inspectLocalFiles(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	directory, _, err := openLocalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	cause := errors.New("final import reserve query failed")
	probe := func(repository string) (uint64, uint64, error) {
		if _, err := os.Stat(filepath.Join(repository, ".git", "refs", "pangu-import", "final-check")); err == nil {
			return 0, 0, cause
		}
		return 50 << 30, 100 << 30, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = runLocalImportWithGuard(ctx, repository, root, parent, "refs/pangu-import/final-check", directory, manifest, probe, localGitCommand)
	if !errors.Is(err, cause) {
		t.Fatalf("successful import omitted final reserve query: %v", err)
	}
	if _, err := git(context.Background(), repository, "rev-parse", "--verify", "refs/pangu-import/final-check"); err != nil {
		t.Fatalf("test never reached completed import: %v", err)
	}
}
