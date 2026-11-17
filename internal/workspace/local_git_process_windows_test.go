package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalGitProcessChild(t *testing.T) {
	marker := os.Getenv("PANGU_LOCAL_GIT_PROCESS_MARKER")
	if marker == "" {
		return
	}
	if err := os.WriteFile(marker, []byte("unexpected execution"), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestLocalGitJobAssignmentFailureNeverResumesChild(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "resumed")
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestLocalGitProcessChild$")
	cmd.Env = append(os.Environ(), "PANGU_LOCAL_GIT_PROCESS_MARKER="+marker)
	if _, err := startLocalGitInJob(cmd, &localGitJob{handle: 0}); err == nil || !strings.Contains(err.Error(), "assign owned Git Job:") {
		t.Fatalf("Job assignment failure was not exercised: %v", err)
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatal("suspended child was not reaped")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("uncontained child resumed: %v", err)
	}
}

func TestLocalGitJobStartupAndStopConfirmationErrorsFailClosed(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), filepath.Join(t.TempDir(), "missing.exe"))
	if _, err := startLocalGit(cmd); err == nil {
		t.Fatal("nonexistent executable accepted")
	}
	if err := waitLocalGitJob(windows.InvalidHandle); err == nil {
		t.Fatal("invalid handle treated as confirmed stopped")
	}
	job, err := newLocalGitJob()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CloseHandle(job.handle); err != nil {
		t.Fatal(err)
	}
	if err := job.stop(); err == nil {
		t.Fatal("unconfirmed Job stop accepted")
	}
}
