package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func observedForkFixture(t *testing.T) (rebuildFixture, string, *workflow.Fork) {
	t.Helper()
	f := newRebuildFixture(t)
	node := uuid.NewString()
	_, err := Execute(f.ctx, f.run, node, 1, true, func(string) (workflow.NodeOutput, error) {
		report := workflow.Report{ID: uuid.NewString(), Title: "source report", Markdown: "verified source body"}
		return workflow.NodeOutput{Text: "source output", RunID: node, ReportIDs: []string{report.ID}, Reports: []workflow.Report{report}, ReportSHA256: workflow.ReportHash(report.Markdown)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	commit, out, err := Checkpoint(f.ctx, f.run, node)
	if err != nil {
		t.Fatal(err)
	}
	target := uuid.NewString()
	if err = EnsureFork(f.ctx, target, f.project.ID, f.namespace, f.run, commit); err != nil {
		t.Fatal(err)
	}
	fork := &workflow.Fork{Reused: map[string]workflow.NodeOutput{"build": out}, Checkpoints: map[string]string{"build": commit}}
	return f, target, fork
}

func TestObservedForkRejectsBeforeCheckpointPublication(t *testing.T) {
	f, target, fork := observedForkFixture(t)
	rejected := errors.New("proof refused")
	_, err := ImportCheckpointsObserved(f.ctx, target, fork, []string{"build"}, func(_ context.Context, node, commit string, out *workflow.NodeOutput, cached bool) error {
		if cached || node != "build" || commit == "" || out.RunID == fork.Reused[node].RunID {
			t.Fatal("observer did not see actual import")
		}
		if len(out.Reports) != 1 || out.ReportIDs[0] == fork.Reused[node].ReportIDs[0] || out.Reports[0].Markdown != "verified source body" {
			t.Fatal("observer did not see final remapped report")
		}
		return rejected
	})
	if !errors.Is(err, rejected) {
		t.Fatalf("observer rejection lost: %v", err)
	}
	var count int
	if err = f.conn.QueryRow(f.ctx, "SELECT count(*) FROM pangu.workspace_checkpoints WHERE run_id=$1", target).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unproven checkpoint published")
	}
	if err = f.conn.QueryRow(f.ctx, "SELECT count(*) FROM pangu.research_reports WHERE run_id=$1", target).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unproven report published")
	}
}

func TestObservedForkRevalidatesCachedInsteadOfBlessingIt(t *testing.T) {
	f, target, fork := observedForkFixture(t)
	if _, err := ImportCheckpoints(f.ctx, target, fork); err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("legacy cached result has no proof")
	called := false
	_, err := ImportCheckpointsObserved(f.ctx, target, fork, []string{"build"}, func(_ context.Context, _, _ string, out *workflow.NodeOutput, cached bool) error {
		if !cached || out.Lineage != nil {
			t.Fatal("expected legacy cache")
		}
		called = true
		return rejected
	})
	if !called || !errors.Is(err, rejected) {
		t.Fatalf("cached import bypassed observer: %v", err)
	}
}
