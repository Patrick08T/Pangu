package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func breakRebuildSource(t *testing.T, f rebuildFixture, source, name string) {
	t.Helper()
	switch name {
	case "source-worker":
		f.sql(t, "UPDATE pangu.workspace_runs SET worker_id='foreign' WHERE run_id=$1", source)
	case "source-directory":
		f.sql(t, "UPDATE pangu.workspace_runs SET directory='foreign' WHERE run_id=$1", source)
	case "expired":
		f.sql(t, "UPDATE pangu.workspace_runs SET expired_at=now() WHERE run_id=$1", source)
	case "unready":
		f.sql(t, "UPDATE pangu.workspace_runs SET ready=false WHERE run_id=$1", source)
	case "missing-git":
		record, err := Get(f.ctx, source)
		if err != nil || record.Directory != filepath.Join(f.directory, "w", source) {
			t.Fatal("refuse to alter Git outside the fixture", record, err)
		}
		base := filepath.Join(record.Directory, "base")
		if err = os.Rename(filepath.Join(base, ".git"), filepath.Join(base, ".git-hidden")); err != nil {
			t.Fatal(err)
		}
	case "missing-submission":
		f.sql(t, "DELETE FROM pangu.submissions WHERE run_id=$1", source)
	case "missing-origin":
		f.sql(t, "UPDATE pangu.submissions SET snapshot=snapshot #- '{fork,origin_commit}' WHERE run_id=$1", source)
	case "invalid-origin":
		f.sql(t, "UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{fork,origin_commit}','\"not-a-commit\"') WHERE run_id=$1", source)
	case "nonancestor-origin":
		record, err := Get(f.ctx, source)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Join(record.Directory, "base")
		tree := rebuildGit(t, f.ctx, base, "rev-parse", "HEAD^{tree}")
		other := rebuildGit(t, f.ctx, base, "commit-tree", tree, "-m", "unrelated root")
		f.sql(t, "UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{fork,origin_commit}',to_jsonb($2::text)) WHERE run_id=$1", source, other)
	case "perforce":
		f.sql(t, "UPDATE pangu.projects SET vcs='perforce',perforce='{}'::jsonb WHERE id=$1", f.project.ID)
	default:
		t.Fatalf("unknown fixture corruption %q", name)
	}
}

func rebuildSourceState(t *testing.T, f rebuildFixture) string {
	t.Helper()
	parts := []string{}
	queries := []struct{ table, filter, value string }{
		{"workspace_runs", "project_id", f.project.ID},
		{"submissions", "project_id", f.project.ID},
		{"run_cancellations", "namespace", f.namespace},
	}
	for _, query := range queries {
		var rows string
		err := f.conn.QueryRow(f.ctx, "SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb)::text FROM pangu."+query.table+" r WHERE "+query.filter+"=$1", query.value).Scan(&rows)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, rows)
	}
	entries, err := os.ReadDir(filepath.Join(f.directory, "w"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		parts = append(parts, entry.Name())
	}
	return strings.Join(parts, "\n")
}

func assertRebuildSourceLeaseReleased(t *testing.T, f rebuildFixture, source string) {
	t.Helper()
	var locked bool
	if err := f.conn.QueryRow(f.ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", "retention:"+source).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Fatal("read-only validation leaked its source retention lease")
	}
	f.sql(t, "SELECT pg_advisory_unlock(hashtextextended($1,0))", "retention:"+source)
}
