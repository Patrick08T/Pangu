package workspace

import (
	"context"
	"errors"
	"testing"
)

func importGuardRefs(t *testing.T) (string, string, string, string) {
	t.Helper()
	repository, parent := localTestRepository(t)
	temporary, final := "refs/pangu-import/guard", "refs/heads/local-guard"
	for _, ref := range []string{temporary, final} {
		if _, err := git(context.Background(), repository, "update-ref", ref, parent); err != nil {
			t.Fatal(err)
		}
	}
	return repository, parent, temporary, final
}

func assertImportGuardRef(t *testing.T, repository, ref, expected string) {
	t.Helper()
	got, err := git(context.Background(), repository, "rev-parse", "--verify", ref)
	if err != nil || got != expected {
		t.Fatalf("unsafe cleanup changed protected ref %s: got=%q err=%v", ref, got, err)
	}
}

func TestLocalImportGuardPriorUnknownStopPreservesBothReferences(t *testing.T) {
	localOperationFixture(t)
	repository, parent, temporary, final := importGuardRefs(t)
	original := errors.New("original import failure")
	cause := errors.Join(original, errLocalGitStopUnconfirmed)
	err := cleanupLocalImport(repository, temporary, final, parent, cause)
	if !errors.Is(err, original) || !errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("cleanup lost original/stop cause: %v", err)
	}
	assertImportGuardRef(t, repository, temporary, parent)
	assertImportGuardRef(t, repository, final, parent)
}
