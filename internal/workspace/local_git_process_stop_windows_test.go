package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalGitInvalidJobStopIsIdentifiablyUnconfirmed(t *testing.T) {
	job, err := newLocalGitJob()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CloseHandle(job.handle); err != nil {
		t.Fatal(err)
	}
	for name, stop := range map[string]func() error{
		"query": func() error { return waitLocalGitJob(windows.InvalidHandle) },
		"stop":  job.stop,
	} {
		t.Run(name, func(t *testing.T) {
			err := stop()
			var errno syscall.Errno
			if !errors.Is(err, errLocalGitStopUnconfirmed) || !errors.Is(err, windows.ERROR_INVALID_HANDLE) || !errors.As(err, &errno) {
				t.Fatalf("invalid owned Job lost unknown marker/OS error: %v", err)
			}
		})
	}
}

func TestLocalGitNeverStartedChildDoesNotBecomeUnknown(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), filepath.Join(t.TempDir(), "missing.exe"))
	_, err := startLocalGitInJob(cmd, &localGitJob{handle: windows.InvalidHandle})
	if err == nil || !errors.Is(err, os.ErrNotExist) || errors.Is(err, errLocalGitStopUnconfirmed) || cmd.Process != nil {
		t.Fatalf("unstarted child falsely marked running/unknown: %v", err)
	}
}
