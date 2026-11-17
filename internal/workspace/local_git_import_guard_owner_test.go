package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestLocalImportGuardUnknownCleanupPoisonsOwningOperation(t *testing.T) {
	root := localOperationFixture(t)
	repository, parent, temporary, final := importGuardRefs(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := errors.New("lost import stop confirmation")
	cause := errors.Join(original, errLocalGitStopUnconfirmed)
	err = cleanupLocalImportContext(ctx, repository, temporary, final, parent, cause)
	if !errors.Is(err, original) || !errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("cleanup hid owner poison: %v", err)
	}
	if err := finish(nil); !errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("cleanup did not record persistent owner poison: %v", err)
	}
	if len(localOperationEntries(t, root)) != 1 {
		t.Fatal("unknown cleanup cleared worker guard")
	}
	assertImportGuardRef(t, repository, temporary, parent)
	assertImportGuardRef(t, repository, final, parent)
}

func TestLocalImportGuardCanceledCleanupRetainsLiveOwner(t *testing.T) {
	root := localOperationFixture(t)
	repository, parent, temporary, final := importGuardRefs(t)
	owner, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(owner)
	cancel()
	cause := errors.New("publication canceled after confirmed stop")
	err = cleanupLocalImportContext(ctx, repository, temporary, final, parent, cause)
	if !errors.Is(err, cause) || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("confirmed cleanup failed: %v", err)
	}
	for _, ref := range []string{temporary, final} {
		if _, err := git(context.Background(), repository, "rev-parse", "--verify", ref); err == nil {
			t.Fatal("owner was lost during canceled cleanup", ref)
		}
	}
	if len(localOperationEntries(t, root)) != 1 || checkLocalGitOperation(owner) != nil {
		t.Fatal("nested cleanup retired outer owner")
	}
	if err := finish(cause); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	if len(localOperationEntries(t, root)) != 0 {
		t.Fatal("confirmed canceled cleanup left quarantine")
	}
}
