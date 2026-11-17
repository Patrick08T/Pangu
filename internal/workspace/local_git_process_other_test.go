//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package workspace

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestLocalGitUnsupportedStartNeverMarksStopUnknown(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "never-start-local-git")
	stop, err := startLocalGit(cmd)
	if err == nil || stop != nil || cmd.Process != nil || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("unsupported platform falsely reports a running orphan: %v", err)
	}
}
