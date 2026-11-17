package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPerforceOperationDocumentKeepsSixtyFourKiBBound(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	repository := filepath.Join(f.run.Directory, "base")
	for _, size := range []int{64 * 1024, 64*1024 + 1} {
		body := bytes.Repeat([]byte("x"), size)
		if err := os.WriteFile(filepath.Join(repository, "bounded.txt"), body, 0600); err != nil {
			t.Fatal(err)
		}
		largeRecoveryGit(t, repository, "add", "--", "bounded.txt")
		largeRecoveryGit(t, repository, "commit", "-m", "document boundary fixture")
		commit := largeRecoveryGit(t, repository, "rev-parse", "HEAD")
		if _, err := f.conn.Exec(ctx, `UPDATE pangu.workspace_runs SET base_commit=$2,head_commit=$2 WHERE run_id=$1`, f.run.ID, commit); err != nil {
			t.Fatal(err)
		}
		if _, err := f.conn.Exec(ctx, `UPDATE pangu.perforce_sources SET local_commit=$2 WHERE run_id=$1`, f.run.ID, commit); err != nil {
			t.Fatal(err)
		}
		data, base, err := Document(ctx, f.run.ID, "bounded.txt")
		if size == 64*1024 && (err != nil || base != commit || !bytes.Equal(data, body)) {
			t.Fatalf("exact 64KiB document changed: bytes=%d base=%s error=%v", len(data), base, err)
		}
		if size > 64*1024 && (err == nil || data != nil) {
			t.Fatalf("oversized document returned data: bytes=%d error=%v", len(data), err)
		}
	}
}
