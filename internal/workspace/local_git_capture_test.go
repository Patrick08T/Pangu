package workspace

import (
	"context"
	"encoding/binary"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func localTestLargeFile(t *testing.T, root string, megabytes int) {
	t.Helper()
	file, err := os.Create(filepath.Join(root, "data.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	buffer := make([]byte, 1<<20)
	rng := rand.New(rand.NewPCG(7, 19))
	for i := 0; i < len(buffer); i += 8 {
		binary.LittleEndian.PutUint64(buffer[i:i+8], rng.Uint64())
	}
	for i := 0; i < megabytes; i++ {
		if _, err := file.Write(buffer); err != nil {
			t.Fatal(err)
		}
	}
}

func localTestObjectBytes(t *testing.T, repository string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(filepath.Join(repository, ".git", "objects"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err == nil {
			total += info.Size()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func TestLocalGitUnchangedSecondCaptureReusesObjects(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	localTestLargeFile(t, root, 16)
	m, err := inspectLocalFiles(context.Background(), root, []string{"data.bin"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := importLocalFiles(context.Background(), repository, root, parent, m)
	if err != nil {
		t.Fatal(err)
	}
	before := localTestObjectBytes(t, repository)
	second, err := importLocalFiles(context.Background(), repository, root, first, m)
	if err != nil {
		t.Fatal(err)
	}
	after := localTestObjectBytes(t, repository)
	a, _ := git(context.Background(), repository, "rev-parse", first+"^{tree}")
	b, _ := git(context.Background(), repository, "rev-parse", second+"^{tree}")
	if a != b || after-before > 1<<20 {
		t.Fatalf("unchanged objects duplicated: tree %s/%s bytes %d -> %d", a, b, before, after)
	}
	t.Logf("unchanged second capture retained object bytes: %d -> %d (delta %d)", before, after, after-before)
}

func TestLocalGitCanceledPartialImportNeverPublishes(t *testing.T) {
	repository, parent := localTestRepository(t)
	root := t.TempDir()
	localTestLargeFile(t, root, 256)
	m, err := inspectLocalFiles(context.Background(), root, []string{"data.bin"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := importLocalFiles(ctx, repository, root, parent, m); done <- err }()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("import ended before partial cancellation: %v", err)
		case <-deadline.C:
			cancel()
			<-done
			t.Fatal("no partial object observed")
		case <-ticker.C:
			if localTestHasPartialPack(repository) {
				cancel()
				err := <-done
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel cause: %v", err)
				}
				assertLocalNoPublishedImport(t, repository, parent)
				return
			}
		}
	}
}

func localTestHasPartialPack(repository string) bool {
	entries, err := os.ReadDir(filepath.Join(repository, ".git", "objects", "pack"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && !entry.IsDir() && strings.Contains(entry.Name(), "pack") && info.Size() > 64<<10 {
			return true
		}
	}
	return false
}
