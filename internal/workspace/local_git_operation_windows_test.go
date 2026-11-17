package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalGitOperationRealJobQueryUnknownRetainsGuard(t *testing.T) {
	root := localOperationFixture(t)
	owner, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cause := waitLocalGitJob(windows.InvalidHandle)
	if !errors.Is(cause, windows.ERROR_INVALID_HANDLE) || !errors.Is(cause, errLocalGitStopUnconfirmed) {
		t.Fatalf("fixture did not produce actual OS stop-query uncertainty: %v", cause)
	}
	recordLocalGitOperationError(owner, cause)
	err = finish(nil)
	if !errors.Is(err, windows.ERROR_INVALID_HANDLE) || len(localOperationEntries(t, root)) != 1 {
		t.Fatalf("lost OS evidence or durable guard: %v", err)
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("fresh attempt ignored actual OS stop uncertainty")
	}
}

func TestLocalGitOperationDeleteFailureRemainsFailClosed(t *testing.T) {
	root := localOperationFixture(t)
	_, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	entries := localOperationEntries(t, root)
	if len(entries) != 1 {
		t.Fatal("missing persistent guard")
	}
	name, err := windows.UTF16PtrFromString(filepath.Join(root, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	// This own-file sharing handle prevents deletion without changing any ACL.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := finish(nil); err == nil || len(localOperationEntries(t, root)) != 1 {
		t.Fatalf("delete failure silently cleared quarantine: %v", err)
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("undeleted guard admitted a new operation")
	}
}
