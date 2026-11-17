package workspace

import (
	"errors"
	"os/exec"
)

var errLocalGitStopUnconfirmed = errors.New("本地Git停止状态未确认")

func localGitStopError(err error) error {
	if err == nil || errors.Is(err, errLocalGitStopUnconfirmed) {
		return err
	}
	return errors.Join(err, errLocalGitStopUnconfirmed)
}

func localGitReapError(cmd *exec.Cmd, cause error) error {
	// A nonzero exit (including a signal) is known stopped once Wait returned state.
	if cmd.ProcessState == nil {
		return errors.Join(cause, errLocalGitStopUnconfirmed)
	}
	return cause
}

func runManagedLocalGit(cmd *exec.Cmd) error {
	stop, err := startLocalGit(cmd)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	return errors.Join(err, stop())
}
