package perforce

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() {
	if os.Getenv("PANGU_TEST_P4_CHILD") == "1" {
		fakeP4()
		os.Exit(0)
	}
	if os.Getenv("PANGU_TEST_P4_HOLDER") == "1" {
		time.Sleep(4 * time.Second)
		os.Exit(0)
	}
}

func fakeP4() {
	args := os.Args[1:]
	if fakeInventory(args) || fakeTrack(args) {
		return
	}
	if slices.Contains(args, "info") {
		fakeP4Info(args)
		return
	}
	if err := fakeP4Contract(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(7)
	}
	mode := os.Getenv("PANGU_TEST_P4_MODE")
	if mode == "third" && slices.Contains(args, "reconcile") {
		mode = "severity"
	}
	_ = os.WriteFile(filepath.Join(os.Getenv("PANGU_TEST_P4_ROOT"), "process.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
	if fakeP4StreamFailure(mode) {
		return
	}
	if mode == "wait" {
		time.Sleep(time.Minute)
		return
	}
	if mode == "holder" {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "PANGU_TEST_P4_CHILD=0", "PANGU_TEST_P4_HOLDER=1")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(8)
		}
		_ = os.WriteFile(filepath.Join(os.Getenv("PANGU_TEST_P4_ROOT"), "holder.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600)
		return
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if mode == "large" {
		for i := 0; i < 100000; i++ {
			fmt.Fprintf(w, "{\"clientFile\":\"//daily/项目/z.txt\",\"padding\":\"%s\"}\r\n", strings.Repeat("p", 64))
		}
		fmt.Fprint(w, `{"clientFile":"//daily/项目/a.txt"}`)
		return
	}
	if mode == "pair" {
		fmt.Fprint(w, `{"clientFile":"//daily/项目/z.txt"}{"clientFile":"//daily/项目/a.txt"}{"clientFile":"//daily/项目/z.txt"}`)
		return
	}
	fmt.Fprint(w, `{"clientFile":"//daily/项目/ok.txt"}`)
	switch mode {
	case "severity", "errorwait":
		fmt.Fprint(w, `{"severity":3,"data":"late failure"}`)
		if mode == "errorwait" {
			w.Flush()
			time.Sleep(time.Minute)
		}
	case "invalid":
		fmt.Fprint(w, `{"clientFile":`)
	case "unsafe":
		fmt.Fprint(w, `{"clientFile":"//daily/../escape.txt"}`)
	case "utf8":
		fmt.Fprint(w, `{"clientFile":"//daily/项目/\ufffd.txt"}`)
	case "exit":
		w.Flush()
		os.Exit(9)
	case "record":
		fmt.Fprintf(w, `{"data":"%s"}`, strings.Repeat("a", 1024*1024))
	}
}

func fakeP4Info(args []string) {
	want := []string{"-Mj", "-ztag", "-C", "none", "-v", "net.maxwait=30", "-p", "localhost:1666", "-u", "tester", "-c", "daily", "-d", os.Getenv("PANGU_TEST_P4_ROOT"), "info"}
	if !slices.Equal(args, want) {
		os.Exit(7)
	}
	if os.Getenv("PANGU_TEST_P4_MODE") == "ascii-server" {
		fmt.Print(`{}`)
	} else {
		fmt.Print(`{"unicode":"enabled"}`)
	}
}

func fakeP4Contract(args []string) error {
	prefix := []string{"-Mj", "-ztag", "-C", "utf8", "-v", "net.maxwait=30", "-p", "localhost:1666", "-u", "tester", "-c", "daily", "-d", os.Getenv("PANGU_TEST_P4_ROOT"), "-Q", "utf8", "-x", "-"}
	if len(args) < len(prefix) || !slices.Equal(args[:len(prefix)], prefix) {
		return fmt.Errorf("unexpected argv %v", args)
	}
	cwd, _ := os.Getwd()
	if !strings.EqualFold(cwd, os.Getenv("PANGU_TEST_P4_ROOT")) {
		return fmt.Errorf("wrong cwd %s", cwd)
	}
	for _, key := range []string{"P4CONFIG", "P4CLIENT", "P4PORT", "P4USER", "P4PASSWD"} {
		if os.Getenv(key) != "" {
			return fmt.Errorf("leaked %s", key)
		}
	}
	if os.Getenv("P4ENVIRO") != os.DevNull {
		return fmt.Errorf("unsafe P4ENVIRO")
	}
	data, _ := io.ReadAll(os.Stdin)
	if string(data) != "//daily/项目/...\n" {
		return fmt.Errorf("wrong Unicode input %q", data)
	}
	for _, want := range [][]string{{"fstat", "-Rh", "-T", "clientFile"}, {"fstat", "-Ro", "-T", "clientFile"}, {"reconcile", "-n", "-a", "-l"}} {
		if slices.Equal(args[len(args)-len(want):], want) {
			return nil
		}
	}
	return fmt.Errorf("unexpected command %v", args)
}

func fakeSnapshot(t *testing.T, mode string) Snapshot {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "项目"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_P4_BINARY", binary)
	t.Setenv("PANGU_TEST_P4_CHILD", "1")
	t.Setenv("PANGU_TEST_P4_MODE", mode)
	t.Setenv("PANGU_TEST_P4_ROOT", root)
	for _, key := range []string{"P4CONFIG", "P4ENVIRO", "P4CLIENT", "P4PORT", "P4USER", "P4PASSWD"} {
		t.Setenv(key, "must-not-leak")
	}
	return Snapshot{Config: Config{Port: "localhost:1666", User: "tester", Client: "daily", Scope: "项目"}, Client: "daily", LocalRoot: root}
}

func TestLocalFilesStreamsLargeListingWithoutLosingDuplicatesOrSort(t *testing.T) {
	s := fakeSnapshot(t, "large")
	got, err := s.LocalFiles(t.Context())
	if err != nil || !slices.Equal(got, []string{"a.txt", "z.txt"}) {
		t.Fatalf("large local listing: got=%v err=%v", got, err)
	}
}

func TestLocalFilesNeverReturnsPartialListOnLateFailure(t *testing.T) {
	for _, mode := range []string{"severity", "invalid", "unsafe", "utf8", "exit", "record", "third"} {
		t.Run(mode, func(t *testing.T) {
			s := fakeSnapshot(t, mode)
			got, err := s.LocalFiles(t.Context())
			if err == nil || got != nil {
				t.Fatalf("accepted partial listing: got=%v err=%v", got, err)
			}
		})
	}
}

func TestOrdinaryCommandStillRejectsLargeOutput(t *testing.T) {
	s := fakeSnapshot(t, "large")
	got, err := s.localCommand(t.Context(), "fstat", "-Rh", "-T", "clientFile", s.Config.Files(s.Client))
	if err == nil || got != nil || !strings.Contains(err.Error(), "8MiB") {
		t.Fatalf("ordinary cap lost: rows=%d err=%v", len(got), err)
	}
}

func TestLocalFilesCancellationAndInheritedPipeAreBounded(t *testing.T) {
	for _, mode := range []string{"wait", "holder", "errorwait"} {
		t.Run(mode, func(t *testing.T) {
			s := fakeSnapshot(t, mode)
			t.Cleanup(func() { stopFakeHolder(t, s.LocalRoot) })
			budget := 5 * time.Second
			if mode == "wait" {
				budget = 200 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(t.Context(), budget)
			defer cancel()
			started := time.Now()
			got, err := s.LocalFiles(ctx)
			if got != nil || err == nil || time.Since(started) > 3*time.Second {
				t.Fatalf("unbounded/accepted canceled process: got=%v err=%v elapsed=%s", got, err, time.Since(started))
			}
			if mode == "errorwait" && !strings.Contains(err.Error(), "late failure") {
				t.Fatalf("lost first writer error: %v", err)
			}
			assertFakeProcessExited(t, s.LocalRoot)
		})
	}
}
