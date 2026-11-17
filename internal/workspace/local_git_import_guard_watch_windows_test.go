package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func importGuardPhaseReady(root, mode string) bool {
	for _, name := range []string{mode + ".pid", "descendant.pid"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			return false
		}
	}
	if mode == "wait" {
		_, err := os.Stat(filepath.Join(root, "waiting"))
		return err == nil
	}
	return true
}

func runImportGuardFixture(t *testing.T, ctx context.Context, mode string, probe localCopySpaceProbe) (error, string, *exec.Cmd) {
	t.Helper()
	localOperationFixture(t)
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	// Entire protocol is <64KiB; with an unread pipe any blocking write is final Flush.
	localTestWrite(t, root, "payload", bytes.Repeat([]byte("x"), 24<<10))
	manifest, err := inspectLocalFiles(context.Background(), root, []string{"payload"})
	if err != nil {
		t.Fatal(err)
	}
	directory, _, err := openLocalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var observed *exec.Cmd
	err = runLocalImportWithGuard(ctx, repository, root, parent, "refs/pangu-import/watch", directory, manifest, probe, importGuardCommand(repository, mode, &observed))
	return err, repository, observed
}

func TestLocalImportGuardIndependentReserveDuringWaitAndFlush(t *testing.T) {
	for _, mode := range []string{"wait", "flush"} {
		t.Run(mode, func(t *testing.T) {
			probe := func(root string) (uint64, uint64, error) {
				if importGuardPhaseReady(root, mode) {
					return (10 << 30) - 1, 100 << 30, nil
				}
				return 50 << 30, 100 << 30, nil
			}
			err, root, cmd := runImportGuardFixture(t, context.Background(), mode, probe)
			assertImportGuardStopped(t, root, mode, cmd)
			if err == nil || !strings.Contains(err.Error(), "保留余量") || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("independent reserve monitor absent during %s: %v", mode, err)
			}
		})
	}
}

func TestLocalImportGuardLiveCallerCauseDuringWaitAndFlush(t *testing.T) {
	for _, mode := range []string{"wait", "flush"} {
		t.Run(mode, func(t *testing.T) {
			cause := errors.New("caller canceled live import")
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			probe := func(root string) (uint64, uint64, error) {
				if importGuardPhaseReady(root, mode) {
					cancel(cause)
				}
				return 50 << 30, 100 << 30, nil
			}
			err, root, cmd := runImportGuardFixture(t, ctx, mode, probe)
			assertImportGuardStopped(t, root, mode, cmd)
			if !errors.Is(err, cause) || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("caller first cause lost during %s: %v", mode, err)
			}
		})
	}
}

func TestLocalImportGuardIndependentQueryFailureDuringWaitAndFlush(t *testing.T) {
	for _, mode := range []string{"wait", "flush"} {
		t.Run(mode, func(t *testing.T) {
			cause := errors.New("import volume query failed")
			probe := func(root string) (uint64, uint64, error) {
				if importGuardPhaseReady(root, mode) {
					return 0, 0, cause
				}
				return 50 << 30, 100 << 30, nil
			}
			err, root, cmd := runImportGuardFixture(t, context.Background(), mode, probe)
			assertImportGuardStopped(t, root, mode, cmd)
			if !errors.Is(err, cause) || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("query first cause lost during %s: %v", mode, err)
			}
		})
	}
}
