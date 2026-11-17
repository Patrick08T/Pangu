package perforce

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeP4StreamFailure(mode string) bool {
	switch mode {
	case "partialwait":
		fmt.Print(`{"clientFile":`)
		_ = os.WriteFile(filepath.Join(os.Getenv("PANGU_TEST_P4_ROOT"), "partial.ready"), nil, 0600)
		time.Sleep(time.Minute)
		return true
	case "diagnosticexit":
		fmt.Print(`{"severity":2,"data":"server diagnostic on stdout"}`)
		os.Exit(9)
	}
	return false
}

func TestLocalFilesCancellationDuringRecordPreservesContextCause(t *testing.T) {
	s := fakeSnapshot(t, "partialwait")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	got, err := s.LocalFiles(ctx)
	if _, readyErr := os.Stat(filepath.Join(s.LocalRoot, "partial.ready")); readyErr != nil {
		t.Fatalf("fixture did not emit partial record: %v", readyErr)
	}
	if got != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost cancellation cause: got=%v err=%v", got, err)
	}
	assertFakeProcessExited(t, s.LocalRoot)
}

func TestLocalFilesPreservesStdoutDiagnosticsOnFailedExit(t *testing.T) {
	s := fakeSnapshot(t, "diagnosticexit")
	got, err := s.LocalFiles(t.Context())
	if got != nil || err == nil || !strings.Contains(err.Error(), "server diagnostic on stdout") {
		t.Fatalf("lost failed-command diagnostic: got=%v err=%v", got, err)
	}
}

func fakeProcess(t *testing.T, root, name string) *os.Process {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if os.IsNotExist(err) && name == "holder.pid" {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	} // Windows may have already released the exited process.
	return process
}

func stopFakeHolder(t *testing.T, root string) {
	t.Helper()
	if p := fakeProcess(t, root, "holder.pid"); p != nil {
		_ = p.Kill()
		_, _ = p.Wait()
	}
}

func assertFakeProcessExited(t *testing.T, root string) {
	t.Helper()
	p := fakeProcess(t, root, "process.pid")
	if p == nil {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = p.Kill()
		<-done
		t.Fatal("command returned without reaping its child")
	}
}

func TestLocalFilesRejectsUnicodeScopeOnNonUnicodeServer(t *testing.T) {
	s := fakeSnapshot(t, "ascii-server")
	got, err := s.LocalFiles(t.Context())
	if got != nil || err == nil || !strings.Contains(err.Error(), "ASCII") {
		t.Fatalf("unsafe charset accepted: got=%v err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(s.LocalRoot, "process.pid")); !os.IsNotExist(err) {
		t.Fatalf("listing ran before charset rejection: %v", err)
	}
}

func TestLocalFileSetRejectsDangerousPaths(t *testing.T) {
	s := fakeSnapshot(t, "unused")
	for _, name := range []string{"//daily/../escape", "//daily/项目/.git/config", "//daily/项目/a...b", "//other/项目/x", "//daily/%zz", "//daily/项目/../../escape"} {
		t.Run(name, func(t *testing.T) {
			files := localFileSet{snapshot: s, limits: localFileLimits{files: 10, paths: 1024}, paths: map[string]struct{}{}}
			if err := files.add(Record{"clientFile": name}); err == nil || len(files.paths) != 0 {
				t.Fatalf("accepted unsafe path %q: %v", name, err)
			}
		})
	}
}
