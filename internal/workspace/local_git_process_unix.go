//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package workspace

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func startLocalGit(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var once sync.Once
	var stopErr error
	stop := func() error {
		once.Do(func() {
			if cmd.Process == nil {
				return
			}
			stopErr = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if errors.Is(stopErr, syscall.ESRCH) {
				stopErr = nil
				return
			}
			if stopErr == nil {
				stopErr = waitLocalGitGroup(cmd.Process.Pid)
			}
			stopErr = localGitStopError(stopErr)
		})
		return stopErr
	}
	cmd.Cancel = stop
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stop, nil
}

func waitLocalGitGroup(pid int) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Kill(-pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return localGitStopError(err)
		}
		if time.Now().After(deadline) {
			return localGitStopError(errors.New("本地Git进程组停止未确认"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
