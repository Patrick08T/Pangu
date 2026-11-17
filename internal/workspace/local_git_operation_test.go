package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func localOperationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("PANGU_WORKSPACE_ROOT", root)
	t.Setenv("PANGU_WORKER_ID", "operation-test-worker")
	return root
}

func localOperationEntries(t *testing.T, root string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestLocalGitOperationPersistsBeforeWorkAndRetainsUnknown(t *testing.T) {
	root := localOperationFixture(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(localOperationEntries(t, root)) != 1 {
		t.Fatal("operation returned before its persistent guard exists")
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("fresh operation crossed the active worker guard")
	}
	cause := errors.Join(context.Canceled, errLocalGitStopUnconfirmed)
	if got := recordLocalGitOperationError(ctx, cause); !errors.Is(got, cause) {
		t.Fatalf("record lost original typed cause: %v", got)
	}
	if err := checkLocalGitOperation(context.WithoutCancel(ctx)); err == nil {
		t.Fatal("poisoned owner was allowed to start another command")
	}
	if err := finish(nil); !errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("outer cause loss erased stop uncertainty: %v", err)
	}
	if len(localOperationEntries(t, root)) != 1 {
		t.Fatal("unknown operation erased persistent guard")
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("fresh owner reopened a quarantined worker")
	}
}

func TestLocalGitOperationNestedOwnerAndCanceledCleanup(t *testing.T) {
	root := localOperationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	nested, end, err := beginLocalGitOperation(context.WithoutCancel(owner))
	if err != nil || checkLocalGitOperation(nested) != nil {
		t.Fatalf("cleanup lost live owner: %v", err)
	}
	if err := end(nil); err != nil || len(localOperationEntries(t, root)) != 1 {
		t.Fatalf("nested command removed outer operation guard: %v", err)
	}
	if err := finish(context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("confirmed cancellation lost cause: %v", err)
	}
	if len(localOperationEntries(t, root)) != 0 {
		t.Fatal("confirmed cancellation leaked worker quarantine")
	}
	if err := checkLocalGitOperation(nested); err == nil {
		t.Fatal("completed owner context could start new commands")
	}
}

func TestLocalGitOperationReplacedGuardIsNeverRemoved(t *testing.T) {
	root := localOperationFixture(t)
	_, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	entries := localOperationEntries(t, root)
	if len(entries) != 1 {
		t.Fatal("missing write-ahead guard")
	}
	path := filepath.Join(root, entries[0].Name())
	if err := os.WriteFile(path, []byte("corrupted guard"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := finish(nil); err == nil {
		t.Fatal("corrupted persistent guard was treated as owned")
	}
	if len(localOperationEntries(t, root)) != 1 {
		t.Fatal("corrupted guard was silently removed")
	}
}

func TestLocalGitOperationMissingRootFailsBeforeWork(t *testing.T) {
	t.Setenv("PANGU_WORKSPACE_ROOT", "")
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("Git operation admitted without a managed worker root")
	}
}
