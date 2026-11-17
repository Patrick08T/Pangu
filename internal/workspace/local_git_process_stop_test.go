package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestLocalGitUnconfirmedReapPreservesFirstCauseAndTypedError(t *testing.T) {
	waitErr := &os.SyscallError{Syscall: "wait", Err: syscall.EINVAL}
	cause := errors.Join(context.Canceled, waitErr)
	got := localGitReapError(&exec.Cmd{}, cause)
	var syscallErr *os.SyscallError
	if !errors.Is(got, errLocalGitStopUnconfirmed) {
		t.Fatalf("unconfirmed reap has no machine-readable marker: %v", got)
	}
	if !errors.Is(got, context.Canceled) || !errors.Is(got, syscall.EINVAL) || !errors.As(got, &syscallErr) || syscallErr != waitErr {
		t.Fatalf("first cause or typed wait error lost: %v", got)
	}
}

func TestLocalGitConfirmedReapDoesNotTreatExitErrorAsUnknown(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestLocalGitStopExitChild$")
	cmd.Env = append(os.Environ(), "PANGU_LOCAL_GIT_STOP_EXIT=1")
	cause := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(cause, &exitErr) || cmd.ProcessState == nil {
		t.Fatalf("nonzero child exit not exercised: %v", cause)
	}
	got := localGitReapError(cmd, cause)
	if errors.Is(got, errLocalGitStopUnconfirmed) || !errors.As(got, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("confirmed nonzero exit marked unknown or lost: %v", got)
	}
}

func TestLocalGitStopExitChild(t *testing.T) {
	if os.Getenv("PANGU_LOCAL_GIT_STOP_EXIT") == "1" {
		os.Exit(7)
	}
}

func TestLocalGitStopErrorPreservesCauseAndNeverMarksSuccess(t *testing.T) {
	if err := localGitStopError(nil); err != nil {
		t.Fatalf("successful stop marked unknown: %v", err)
	}
	cause := &os.SyscallError{Syscall: "owned group", Err: syscall.EPERM}
	got := localGitStopError(cause)
	var typed *os.SyscallError
	if !errors.Is(got, errLocalGitStopUnconfirmed) || !errors.Is(got, syscall.EPERM) || !errors.As(got, &typed) || typed != cause {
		t.Fatalf("unknown stop lost marker or OS cause: %v", got)
	}
}
