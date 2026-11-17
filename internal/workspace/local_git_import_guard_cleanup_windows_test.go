package workspace

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalImportGuardCleanupUnknownStopPreventsFinalDeletion(t *testing.T) {
	localOperationFixture(t)
	repository, parent, temporary, final := importGuardRefs(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = finish(err) }()
	original := errors.New("publication failed")
	// Keep actual update-ref side effects; only the OS stop/query result is faulted.
	run := func(ctx context.Context, directory string, args ...string) (string, error) {
		out, err := runLocalGit(ctx, directory, args...)
		return out, errors.Join(err, waitLocalGitJob(windows.InvalidHandle))
	}
	err = cleanupLocalImportWithGit(ctx, repository, temporary, final, parent, original, run)
	if !errors.Is(err, original) || !errors.Is(err, errLocalGitStopUnconfirmed) || !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("cleanup lost first failure/unknown OS cause: %v", err)
	}
	if _, err := git(context.Background(), repository, "rev-parse", "--verify", temporary); err == nil {
		t.Fatal("fixture did not execute first real cleanup")
	}
	assertImportGuardRef(t, repository, final, parent)
}
