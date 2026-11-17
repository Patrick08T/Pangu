package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/google/uuid"
)

func preparationSource(t *testing.T) (*Run, *perforce.Snapshot) {
	t.Helper()
	_, _, id, _ := localP4Run(t)
	ctx := context.Background()
	run, err := Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	snapshot, err := readPerforce(ctx, conn, id)
	if err != nil {
		t.Fatal(err)
	}
	return run, snapshot
}

func TestP4PreparationFailureReclaimsOnlyOwnedStaging(t *testing.T) {
	run, snapshot := preparationSource(t)
	directory := t.TempDir()
	snapshot.RunID, snapshot.Commit = uuid.NewString(), ""
	keep := filepath.Join(directory, "unrelated-evidence")
	if err := os.WriteFile(keep, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fault := make(chan error, 1)
	go blockPreparationMarker(ctx, directory, fault)
	_, err := prepareLocalObjectRepository(ctx, snapshot, directory, "", run.ProjectID)
	cancel()
	if injectErr := <-fault; injectErr != nil {
		t.Fatalf("fixture did not inject late filesystem failure: %v", injectErr)
	}
	if err == nil {
		t.Fatal("marker publication failure was ignored")
	}
	assertNoPreparationOrphans(t, directory)
	if _, err := prepareLocalObjectRepository(context.Background(), snapshot, directory, "", run.ProjectID); err != nil {
		t.Fatal(err)
	}
	assertNoPreparationOrphans(t, directory)
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatalf("unrelated evidence changed: %q %v", data, err)
	}
}

func blockPreparationMarker(ctx context.Context, directory string, result chan<- error) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		entries, err := os.ReadDir(directory)
		if err != nil {
			result <- err
			return
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "preparing-local-") {
				continue
			}
			metadata := filepath.Join(directory, entry.Name(), ".git")
			if info, err := os.Stat(metadata); err == nil && info.IsDir() {
				result <- os.Mkdir(filepath.Join(metadata, "pangu-local-capture.json"), 0700)
				return
			}
		}
		select {
		case <-ctx.Done():
			result <- ctx.Err()
			return
		case <-ticker.C:
		}
	}
}

func assertNoPreparationOrphans(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "preparing-local-") {
			t.Errorf("unpublished object repository leaked: %s", entry.Name())
		}
	}
}

func TestP4PreparationReusesCompleteCrashOrphan(t *testing.T) {
	run, snapshot := preparationSource(t)
	base := filepath.Join(run.Directory, "base")
	orphan := filepath.Join(run.Directory, "preparing-local-"+uuid.NewString())
	// Both are inside this test-created run; simulate death immediately before publication.
	if err := os.Rename(base, orphan); err != nil {
		t.Fatal(err)
	}
	snapshot.Commit = ""
	commit, err := prepareLocalObjectRepository(context.Background(), snapshot, run.Directory, "", run.ProjectID)
	if err != nil || commit != run.BaseCommit {
		t.Fatalf("completed orphan was not recovered: %q %v", commit, err)
	}
	assertNoPreparationOrphans(t, run.Directory)
	if _, err := os.Stat(filepath.Join(base, ".git")); err != nil {
		t.Fatal(err)
	}
}

func TestP4PreparationRefusesUnconfirmedOrphanWithoutAnotherCopy(t *testing.T) {
	run, snapshot := preparationSource(t)
	directory := t.TempDir()
	snapshot.RunID, snapshot.Commit = uuid.NewString(), ""
	orphan := filepath.Join(directory, "preparing-local-"+uuid.NewString())
	if err := os.Mkdir(orphan, 0700); err != nil {
		t.Fatal(err)
	}
	_, err := prepareLocalObjectRepository(context.Background(), snapshot, directory, "", run.ProjectID)
	if err == nil {
		t.Fatal("unconfirmed incomplete preparation was silently bypassed")
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(orphan) {
		t.Fatalf("allocated another repository or altered orphan: %v %v", entries, readErr)
	}
}
