//go:build windows || linux || darwin || freebsd || openbsd || netbsd || dragonfly

package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalGitOrdinaryFailureAndSuccessNeverMarkStopUnknown(t *testing.T) {
	repository, parent := localTestRepository(t)
	got, err := runLocalGit(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil || got != parent {
		t.Fatalf("normal success: %q %v", got, err)
	}
	_, err = runLocalGit(context.Background(), repository, "not-a-git-command")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("ordinary Git failure confused with stop uncertainty: %v", err)
	}
}

func TestLocalGitConfirmedCancellationNeverMarksStopUnknown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	marker := filepath.Join(t.TempDir(), "started")
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLocalGitStopWaitingChild$")
	cmd.WaitDelay = time.Second
	cmd.Env = append(os.Environ(), "PANGU_LOCAL_GIT_STOP_WAIT="+marker)
	stop, err := startLocalGit(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if ctx.Err() != nil {
			_ = cmd.Wait()
			t.Fatal("owned child never reached running state")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	err = errors.Join(cmd.Wait(), stop())
	if err == nil || cmd.ProcessState == nil || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("confirmed canceled process confused with unknown stop: %v", err)
	}
}

func TestLocalGitStopWaitingChild(t *testing.T) {
	marker := os.Getenv("PANGU_LOCAL_GIT_STOP_WAIT")
	if marker == "" {
		return
	}
	if err := os.WriteFile(marker, []byte("running"), 0600); err != nil {
		os.Exit(2)
	}
	for {
		time.Sleep(time.Second)
	}
}
