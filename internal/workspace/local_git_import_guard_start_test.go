package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalImportGuardInitialProbeFailureWinsOverStartup(t *testing.T) {
	root := t.TempDir()
	cause := errors.New("initial capacity probe failed")
	probe := func(string) (uint64, uint64, error) { return 0, 0, cause }
	factory := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(root, "missing.exe"))
	}
	err := runLocalImportWithGuard(context.Background(), root, root, "", "", nil, localManifest{}, probe, factory)
	if !errors.Is(err, cause) {
		t.Fatalf("process startup preceded required reserve query: %v", err)
	}
}

func TestLocalImportGuardCanceledCallerKeepsCustomCause(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("caller explicitly stopped import")
	cancel(cause)
	probe := func(string) (uint64, uint64, error) { return 50 << 30, 100 << 30, nil }
	factory := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(root, "missing.exe"))
	}
	err := runLocalImportWithGuard(ctx, root, root, "", "", nil, localManifest{}, probe, factory)
	if !errors.Is(err, cause) {
		t.Fatalf("startup masked caller custom cause: %v", err)
	}
}

func TestLocalImportGuardFailedStartupStopsReserveWatcher(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int64
	probe := func(string) (uint64, uint64, error) { calls.Add(1); return 50 << 30, 100 << 30, nil }
	factory := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(root, "missing.exe"))
	}
	err := runLocalImportWithGuard(context.Background(), root, root, "", "", nil, localManifest{}, probe, factory)
	if !errors.Is(err, os.ErrNotExist) || errors.Is(err, errLocalGitStopUnconfirmed) {
		t.Fatalf("startup error identity changed: %v", err)
	}
	initial := calls.Load()
	if initial == 0 {
		t.Fatal("startup skipped initial reserve query")
	}
	time.Sleep(600 * time.Millisecond)
	if calls.Load() != initial {
		t.Fatal("reserve watcher outlived failed startup")
	}
}
