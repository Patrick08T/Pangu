//go:build integration

package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestExperienceErasureWorkspaceRemovesPhysicalCopiesPreservingIdentity(t *testing.T) {
	f := newErasureWorkspaceFixture(t)
	calls := 0
	guard := func(context.Context, pgx.Tx) error { calls++; return nil }
	if err := EraseExperienceRun(context.Background(), f.ns, f.project, f.request, f.run, guard); err != nil {
		t.Fatal("authorized stopped erasure failed", err)
	}
	if _, err := os.Stat(f.directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("private managed directory remains", err)
	}
	var output, body, title, evidence, metadata, snapshot, message string
	var purged bool
	var count int
	err := f.conn.QueryRow(context.Background(), `SELECT c.output::text,convert_from(r.body,'UTF8'),r.title,r.evidence::text,coalesce(r.metadata::text,''),s.snapshot::text,n.message,w.purged_at IS NOT NULL,(SELECT count(*) FROM pangu.retention_records x WHERE x.run_id=w.run_id AND x.status='purged') FROM pangu.workspace_checkpoints c JOIN pangu.research_reports r ON r.node_run_id=c.node_run_id JOIN pangu.workspace_runs w ON w.run_id=c.run_id JOIN pangu.submissions s ON s.run_id=w.run_id JOIN pangu.node_attempts n ON n.node_run_id=c.node_run_id WHERE c.node_run_id=$1`, f.node).Scan(&output, &body, &title, &evidence, &metadata, &snapshot, &message, &purged, &count)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" || title != "" || evidence != "[]" || metadata != "" || message != "" || strings.Contains(output+snapshot, "private") || !purged || count != 1 || calls < 2 {
		t.Fatalf("private payload/tombstone mismatch %q %q %q %q %q %q %q %v %d %d", output, body, title, evidence, metadata, snapshot, message, purged, count, calls)
	}
	if !strings.Contains(output, f.node) || !strings.Contains(output, strings.Repeat("d", 64)) || !strings.Contains(output, "experience_erased") {
		t.Fatal("immutable output identity/hash lost", output)
	}
	if err = EraseExperienceRun(context.Background(), f.ns, f.project, f.request, f.run, guard); err != nil {
		t.Fatal("erasure not idempotent", err)
	}
}

func TestExperienceErasureWorkspaceUnknownStopOrUnsafeScopeNeverDeletes(t *testing.T) {
	for _, mode := range []string{"no-history", "running-child", "active", "no-cancel", "foreign-worker", "foreign-project", "path", "no-authority", "nil-verifier"} {
		t.Run(mode, func(t *testing.T) {
			f := newErasureWorkspaceFixture(t)
			project := f.project
			guard := ExperienceErasureVerifier(func(context.Context, pgx.Tx) error { return nil })
			switch mode {
			case "no-history":
				f.exec(t, `DELETE FROM pangu.run_history WHERE run_id=$1`, f.run)
			case "running-child":
				f.exec(t, `UPDATE pangu.run_history SET summary=jsonb_set(summary,'{nodes,0,status}','"running"') WHERE run_id=$1`, f.run)
			case "active":
				f.exec(t, `UPDATE pangu.node_attempts SET finished_at=NULL,status='running' WHERE run_id=$1`, f.run)
			case "no-cancel":
				f.exec(t, `DELETE FROM pangu.run_cancellations WHERE run_id=$1`, f.run)
			case "foreign-worker":
				f.exec(t, `UPDATE pangu.workspace_runs SET worker_id='foreign' WHERE run_id=$1`, f.run)
			case "foreign-project":
				project = "00000000-0000-0000-0000-000000000001"
			case "path":
				f.exec(t, `UPDATE pangu.workspace_runs SET directory=$2 WHERE run_id=$1`, f.run, filepath.Join(f.root, "outside"))
			case "no-authority":
				guard = func(context.Context, pgx.Tx) error { return errors.New("cause no longer proven") }
			case "nil-verifier":
				guard = nil
			}
			if err := EraseExperienceRun(context.Background(), f.ns, project, f.request, f.run, guard); err == nil {
				t.Fatal("unproven erasure acknowledged")
			}
			if _, err := os.Stat(filepath.Join(f.directory, "private.txt")); err != nil {
				t.Fatal("unproven erasure removed file", err)
			}
			var intact bool
			if err := f.conn.QueryRow(context.Background(), `SELECT octet_length(body)>0 FROM pangu.research_reports WHERE id=$1`, f.report).Scan(&intact); err != nil || !intact {
				t.Fatal("unproven erasure removed body", err)
			}
		})
	}
}
