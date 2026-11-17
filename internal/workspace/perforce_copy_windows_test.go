package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestPerforceCopyRejectsActualJunction(t *testing.T) {
	for _, name := range []string{"objects/junction", "refs", "config", "HEAD"} {
		t.Run(name, func(t *testing.T) {
			root, outside := copyTestStore(t), t.TempDir()
			localTestWrite(t, outside, "sentinel", []byte("untouched"))
			link := filepath.Join(root, ".git", filepath.FromSlash(name))
			cmd := exec.Command("cmd", "/c", "mklink", "/J", link, outside)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("junction fixture: %s %v", out, err)
			}
			defer os.Remove(link)
			if _, err := inspectLocalObjectStore(context.Background(), root); err == nil {
				t.Fatal("Git metadata junction accepted")
			}
			body, err := os.ReadFile(filepath.Join(outside, "sentinel"))
			if err != nil || string(body) != "untouched" {
				t.Fatalf("outside metadata target changed: %q %v", body, err)
			}
		})
	}
}

func TestPerforceCopyQuietChild(t *testing.T) {
	root := os.Getenv("PANGU_COPY_QUIET_ROOT")
	if root == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(root, "child.pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(7)
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func assertCopyChildStopped(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "child.pid"))
	if err != nil {
		t.Fatal("quiet descendant never started", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return
	}
	if err != nil {
		t.Fatal("cannot prove descendant stopped", err)
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant still active: state=%d err=%v", state, err)
	}
}

func quietLocalCopyArgs(t *testing.T) (string, []string) {
	t.Helper()
	root, _ := localTestRepository(t)
	t.Setenv("PANGU_COPY_QUIET_ROOT", root)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	alias := "alias.copyquiet=!" + "'" + strings.ReplaceAll(filepath.ToSlash(binary), "'", "'\"'\"'") + "' -test.run=^TestPerforceCopyQuietChild$"
	return root, []string{"-c", alias, "copyquiet"}
}

func TestPerforceCopyReserveCancelsQuietGitAndWaitsForDescendant(t *testing.T) {
	root, args := quietLocalCopyArgs(t)
	probe := func(string) (uint64, uint64, error) {
		if _, err := os.Stat(filepath.Join(root, "child.pid")); err == nil {
			return (10 << 30) - 1, 100 << 30, nil
		}
		return 50 << 30, 100 << 30, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := runLocalCopyCommand(ctx, root, probe, args...)
	if err == nil || !strings.Contains(err.Error(), "保留余量") {
		t.Fatalf("lost reserve cause: %v", err)
	}
	assertCopyChildStopped(t, root)
}

func TestPerforceCopyLiveProbeFailureAndCallerCauseStopDescendant(t *testing.T) {
	for _, mode := range []string{"probe", "caller"} {
		t.Run(mode, func(t *testing.T) {
			root, args := quietLocalCopyArgs(t)
			cause := errors.New("original-" + mode + "-cause")
			parent, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			ctx, cancel := context.WithCancelCause(parent)
			defer cancel(nil)
			probe := func(string) (uint64, uint64, error) {
				if _, err := os.Stat(filepath.Join(root, "child.pid")); err == nil {
					if mode == "caller" {
						cancel(cause)
					} else {
						return 0, 0, cause
					}
				}
				return 50 << 30, 100 << 30, nil
			}
			err := runLocalCopyCommand(ctx, root, probe, args...)
			if !errors.Is(err, cause) {
				t.Fatalf("lost original cause: %v", err)
			}
			assertCopyChildStopped(t, root)
		})
	}
}

func TestPerforceCopyProbeFailurePreventsProcessStart(t *testing.T) {
	root, _ := localTestRepository(t)
	cause := fmt.Errorf("disk probe offline")
	probe := func(string) (uint64, uint64, error) { return 0, 0, cause }
	err := runLocalCopyCommand(context.Background(), root, probe, "--version")
	if !errors.Is(err, cause) {
		t.Fatalf("space query failure was ignored: %v", err)
	}
}
