package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestRebuildSourceRejectsUnavailableOrForeignSourceWithoutWrites(t *testing.T) {
	cases := []string{"empty", "invalid-id", "namespace", "project", "worker", "source-worker", "source-directory", "expired", "unready", "missing-git", "missing-submission", "missing-origin", "invalid-origin", "nonancestor-origin", "perforce"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRebuildFixture(t)
			source := f.fork(t)
			project, namespace := f.project.ID, f.namespace
			switch name {
			case "empty":
				source = ""
			case "invalid-id":
				source = "not-a-uuid"
			case "namespace":
				namespace = "foreign"
			case "project":
				project = uuid.NewString()
			case "worker":
				t.Setenv("PANGU_WORKER_ID", "foreign")
			default:
				breakRebuildSource(t, f, source, name)
			}
			before := rebuildSourceState(t, f)
			origin, err := RebuildSource(f.ctx, source, project, namespace)
			if err == nil || origin != "" {
				t.Errorf("unavailable source returned origin %q, error %v", origin, err)
			}
			if after := rebuildSourceState(t, f); after != before {
				t.Error("read-only validation changed database state or workspace directories")
			}
			assertRebuildSourceLeaseReleased(t, f, source)
		})
	}
}

func TestRebuildSourceTraversesForkRerunReadOnly(t *testing.T) {
	f := newRebuildFixture(t)
	fork := f.fork(t)
	rerun := uuid.NewString()
	f.submission(t, rerun, map[string]any{"source_run_id": fork})
	if err := EnsureRerun(f.ctx, rerun, f.project.ID, f.namespace, fork); err != nil {
		t.Fatal(err)
	}
	source, err := Get(f.ctx, rerun)
	if err != nil || source.BaseCommit == f.origin {
		t.Fatal("fixture must have a polluted rerun baseline", source, err)
	}
	f.sql(t, "INSERT INTO pangu.run_cancellations(run_id,namespace) VALUES($1,$2)", rerun, f.namespace)
	base := filepath.Join(source.Directory, "base")
	rebuildWrite(t, filepath.Join(base, "untracked.txt"), "must remain")
	head := rebuildGit(t, f.ctx, base, "rev-parse", "HEAD")
	before := rebuildSourceState(t, f)
	for _, id := range []string{f.run, fork, rerun} {
		origin, err := RebuildSource(f.ctx, id, f.project.ID, f.namespace)
		if err != nil || origin != f.origin {
			t.Fatalf("clean origin = %q, want %q, error %v", origin, f.origin, err)
		}
		assertRebuildSourceLeaseReleased(t, f, id)
	}
	if after := rebuildSourceState(t, f); after != before {
		t.Fatal("read-only validation created target state or changed source/cancellation")
	}
	if actual := rebuildGit(t, f.ctx, base, "rev-parse", "HEAD"); actual != head {
		t.Fatal("read-only validation checked out a different source commit")
	}
	for name, want := range map[string]string{"untracked.txt": "must remain", "polluted.txt": "revoked influence"} {
		body, err := os.ReadFile(filepath.Join(base, name))
		if err != nil || string(body) != want {
			t.Fatalf("source file %s changed: %q %v", name, body, err)
		}
	}
}

func TestRebuildSourceDoesNotAuthorizeLaterCheckoutAfterExpiry(t *testing.T) {
	f := newRebuildFixture(t)
	source := f.fork(t)
	if origin, err := RebuildSource(f.ctx, source, f.project.ID, f.namespace); err != nil || origin != f.origin {
		t.Fatal("valid source rejected", origin, err)
	}
	f.sql(t, "UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1", source)
	before := rebuildSourceState(t, f)
	if err := EnsureRebuild(f.ctx, uuid.NewString(), f.project.ID, f.namespace, source); err == nil {
		t.Fatal("checkout trusted an earlier read-only validation after source expiry")
	}
	if after := rebuildSourceState(t, f); after != before {
		t.Fatal("expired source created target workspace state")
	}
}
