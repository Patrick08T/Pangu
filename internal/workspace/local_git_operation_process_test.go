package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalGitOperationCrashPersistsAcrossProcesses(t *testing.T) {
	root := localOperationFixture(t)
	marker := filepath.Join(t.TempDir(), "work-started")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLocalGitOperationCrashChild$")
	cmd.Env = append(os.Environ(), "PANGU_LOCAL_OPERATION_CRASH="+marker)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	var exitErr *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 37 {
		t.Fatalf("helper did not reach intentional crash: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("helper never executed guarded work")
	}
	if len(localOperationEntries(t, root)) != 1 {
		t.Fatal("process crash left no persistent operation guard")
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err == nil {
		t.Fatal("fresh process crossed crashed worker operation")
	}
}

func TestLocalGitOperationCrashChild(t *testing.T) {
	marker := os.Getenv("PANGU_LOCAL_OPERATION_CRASH")
	if marker == "" {
		return
	}
	if _, _, err := beginLocalGitOperation(context.Background()); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(marker, []byte("executed"), 0600); err != nil {
		os.Exit(3)
	}
	os.Exit(37)
}

func TestLocalGitOperationNormalGitLeavesNoQuarantine(t *testing.T) {
	root := localOperationFixture(t)
	repository, parent := localTestRepository(t)
	owner, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := runLocalGit(owner, repository, "rev-parse", "HEAD")
	if err != nil || got != parent {
		t.Fatalf("ordinary Git was not admitted: %q %v", got, err)
	}
	_, err = runLocalGit(owner, repository, "not-a-git-command")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("ordinary command failure confused with unknown: %v", err)
	}
	if err := finish(err); !errors.As(err, &exitErr) {
		t.Fatalf("operation lost command error: %v", err)
	}
	if len(localOperationEntries(t, root)) != 0 {
		t.Fatal("confirmed nonzero exit leaked quarantine")
	}
}
