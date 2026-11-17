//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package workspace

import (
	"errors"
	"os/exec"
)

func startLocalGit(cmd *exec.Cmd) (func() error, error) {
	return nil, errors.New("当前平台不能确认本地Git进程树回收")
}
