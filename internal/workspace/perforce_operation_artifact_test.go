package workspace

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPerforceOperationArtifactQueriesRejectUnknownGitStop(t *testing.T) {
	f := newLargeRecoveryFixture(t)
	ctx := context.Background()
	set, err := Files(ctx, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	object := largeRecoveryGit(t, filepath.Join(f.run.Directory, "base"), "rev-parse", f.run.BaseCommit+":edit.txt")
	queries := map[string]func() error{
		"files":    func() error { _, err := Files(ctx, f.run.ID); return err },
		"size":     func() error { _, err := set.size(ctx, object); return err },
		"blob":     func() error { _, err := set.Blob(ctx, object); return err },
		"diff":     func() error { _, _, err := set.Diff(ctx, "edit.txt"); return err },
		"document": func() error { _, _, err := Document(ctx, f.run.ID, "edit.txt"); return err },
	}
	for name, query := range queries {
		if err := query(); err != nil {
			t.Fatalf("healthy %s query rejected: %v", name, err)
		}
	}
	owner, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recordLocalGitOperationError(owner, errLocalGitStopUnconfirmed)
	_ = finish(nil)
	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			if err := query(); err == nil {
				t.Fatalf("%s query bypassed quarantine", name)
			}
		})
	}
}
