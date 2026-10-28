//go:build !windows

package agent

// Linux 尚未证明宿主崩溃后的完整进程树停止；保留未知状态，不能伪造停止证据。
func ProcessIdentity(int) (string, error) { return "", nil }

func ConfirmStopped(int, string, string) (bool, error) { return false, nil }
