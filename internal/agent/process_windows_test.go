package agent

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRecoveryCannotConfirmAnotherWindowsSession(t *testing.T) {
	stamp, err := ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	created, _, _ := strings.Cut(stamp, "/")
	stopped, err := ConfirmStopped(os.Getpid(), created+"/4294967295", "")
	if err != nil || stopped {
		t.Fatalf("跨 Windows session 被错误确认停止: %t %v", stopped, err)
	}
}

func TestRecoveryRequiresOriginalWorkerToHaveExited(t *testing.T) {
	setupCodex(t)
	cmd := exec.Command(os.Getenv("PANGU_CODEX_BINARY"), "linger")
	cmd.Env = codexEnvironment()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	stamp, err := ProcessIdentity(cmd.Process.Pid)
	if err != nil || stamp == "" {
		t.Fatalf("无法取得进程身份: %v", err)
	}
	stopped, err := ConfirmStopped(cmd.Process.Pid, stamp, "")
	if err != nil || stopped {
		t.Fatalf("活着的 Worker 被声明停止: %t %v", stopped, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	stopped, err = ConfirmStopped(cmd.Process.Pid, stamp, "")
	if err != nil || !stopped {
		t.Fatalf("无法确认已终止的原 Worker: %t %v", stopped, err)
	}
}

func TestJobStopWaitsForNativeExit(t *testing.T) {
	dir := setupCodex(t)
	job, err := codexJob("")
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	cmd := exec.Command(os.Getenv("PANGU_CODEX_BINARY"), "linger")
	cmd.Dir, cmd.Env = dir, codexEnvironment()
	process, err := launchCodex(cmd, job, null, null, null)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process.Process)
	defer windows.CloseHandle(process.Thread)
	if err := stopCodexJob(job); err != nil {
		t.Fatal(err)
	}
	status, err := windows.WaitForSingleObject(process.Process, 0)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("停止返回时，原生进程仍未退出: %d %v", status, err)
	}
}
