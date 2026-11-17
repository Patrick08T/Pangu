package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestExperienceForkRevalidatesReportBodiesAtRecovery(t *testing.T) {
	for _, where := range []string{"source", "target"} {
		t.Run(where, func(t *testing.T) {
			f := newExperienceForkFixture(t)
			f.execute(t, 0)
			f.execute(t, 1)
			r := f.target(t)
			if err := workspace.EnsureFork(f.ctx, r.runID, r.scope.ProjectID, f.ns, f.run, f.snapshot.Fork.Commit); err != nil {
				t.Fatal(err)
			}
			out, err := r.ImportFork(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			report := out["first"].ReportIDs[0]
			if where == "source" {
				report = f.snapshot.Fork.Reused["first"].ReportIDs[0]
			}
			rebuildExec(t, `UPDATE pangu.research_reports SET body='changed bytes' WHERE id=$1`, report)
			if _, err = r.ImportFork(f.ctx); err == nil {
				t.Fatal("cached import accepted changed report")
			}
			if err = r.ValidateReused(f.ctx, out); err == nil {
				t.Fatal("direct reused accepted changed report")
			}
		})
	}
}
