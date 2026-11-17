package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestLocalImportGuardProcessChild(t *testing.T) {
	root, mode := os.Getenv("PANGU_IMPORT_GUARD_ROOT"), os.Getenv("PANGU_IMPORT_GUARD_MODE")
	if root == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(root, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(8)
	}
	if mode != "descendant" {
		child := exec.Command(os.Args[0], "-test.run=^TestLocalImportGuardProcessChild$")
		child.Env = append(os.Environ(), "PANGU_IMPORT_GUARD_MODE=descendant")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		if mode == "wait" {
			if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
				os.Exit(10)
			}
			if err := os.WriteFile(filepath.Join(root, "waiting"), []byte("stdin EOF"), 0600); err != nil {
				os.Exit(11)
			}
		}
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func importGuardCommand(root, mode string, observed **exec.Cmd) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, repository string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLocalImportGuardProcessChild$")
		cmd.Dir, cmd.WaitDelay = repository, time.Second
		cmd.Env = append(os.Environ(), "PANGU_IMPORT_GUARD_ROOT="+root, "PANGU_IMPORT_GUARD_MODE="+mode)
		*observed = cmd
		return cmd
	}
}

func assertImportGuardStopped(t *testing.T, root, mode string, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.ProcessState == nil {
		t.Fatal("owned process was not reaped")
	}
	for _, name := range []string{mode, "descendant"} {
		data, err := os.ReadFile(filepath.Join(root, name+".pid"))
		if err != nil {
			t.Fatal("process never reached test phase", name, err)
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			t.Fatal(err)
		}
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if err != nil {
			t.Fatal("cannot establish process stopped", err)
		}
		state, waitErr := windows.WaitForSingleObject(handle, 0)
		windows.CloseHandle(handle)
		if waitErr != nil || state != windows.WAIT_OBJECT_0 {
			t.Fatalf("import process remains active: pid=%d state=%d err=%v", pid, state, waitErr)
		}
	}
}
