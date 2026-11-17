//go:build integration

package workspace

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestExperienceErasureWorkspaceMissingRowCannotHideOrphanDirectory(t *testing.T) {
	f := newErasureWorkspaceFixture(t)
	f.exec(t, `DELETE FROM pangu.research_reports WHERE run_id=$1`, f.run)
	f.exec(t, `DELETE FROM pangu.workspace_checkpoints WHERE run_id=$1`, f.run)
	f.exec(t, `DELETE FROM pangu.workspace_runs WHERE run_id=$1`, f.run)
	err := EraseExperienceRun(context.Background(), f.ns, f.project, f.request, f.run, func(context.Context, pgx.Tx) error { return nil })
	var purged bool
	if readErr := f.conn.QueryRow(context.Background(), `SELECT purged_at IS NOT NULL FROM pangu.submissions WHERE id=$1`, f.request).Scan(&purged); readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || purged {
		t.Fatal("missing workspace identity acknowledged orphan directory as physically purged", err, purged)
	}
	if _, err = os.Stat(f.directory); err != nil {
		t.Fatal("unknown orphan directory was removed", err)
	}
}
