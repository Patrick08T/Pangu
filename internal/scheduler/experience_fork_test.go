package scheduler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestExperienceForkDerivesChangedHashesAndValidatesDownstream(t *testing.T) {
	f := newExperienceForkFixture(t)
	f.execute(t, 0)
	f.execute(t, 1)
	if err := validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err != nil {
		t.Fatal(err)
	}
	runtime := f.target(t)
	if err := runtime.ValidateFork(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureFork(f.ctx, runtime.runID, runtime.scope.ProjectID, f.ns, f.run, f.snapshot.Fork.Commit); err != nil {
		t.Fatal(err)
	}
	out, err := runtime.ImportFork(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "second"} {
		if out[id].Lineage == nil || out[id].RunID == f.snapshot.Fork.Reused[id].RunID || out[id].Lineage.OutputHash == f.snapshot.Fork.Reused[id].Lineage.OutputHash {
			t.Fatal("import kept old physical proof")
		}
		a, err := runtime.store.ReadArtifact(f.ctx, runtime.scope, out[id].Lineage.ArtifactID)
		if err != nil {
			t.Fatal(err)
		}
		if a.Kind != "checkpoint_import" || len(a.Parents) == 0 || a.Parents[0].ID != f.snapshot.Fork.Reused[id].Lineage.ArtifactID || len(a.References) != 0 {
			t.Fatalf("not a derived artifact: %+v", a)
		}
		if a.ExecutionInputHash == a.InputHash {
			t.Fatal("expected logical input presented as measured import context")
		}
	}
	if err = runtime.ValidateReused(f.ctx, out); err != nil {
		t.Fatal(err)
	}
	input := runtime.request.Snapshot.Workflow.NodeInput(runtime.request.Snapshot.Workflow.Nodes[2], out)
	if err = runtime.Prepare(f.ctx, &input); err != nil {
		t.Fatal(err)
	}
	again, err := runtime.ImportFork(f.ctx)
	before, _ := json.Marshal(out)
	after, _ := json.Marshal(again)
	if err != nil || string(before) != string(after) {
		t.Fatalf("import not stable on recovery: %v", err)
	}
	rebuildExec(t, `UPDATE pangu.memberships SET enabled=false WHERE project_id=$1`, runtime.scope.ProjectID)
	if err = runtime.Prepare(f.ctx, &input); err == nil {
		t.Fatal("downstream escaped revoked source closure")
	}
	if _, err = runtime.ImportFork(f.ctx); err == nil {
		t.Fatal("cached import escaped revoked source")
	}
}

func TestExperienceForkRejectsLegacyForgedAndChangedPrefix(t *testing.T) {
	f := newExperienceForkFixture(t)
	f.execute(t, 0)
	f.execute(t, 1)
	original := f.snapshot.Fork.Reused["first"]
	legacy := original
	legacy.Lineage = nil
	f.snapshot.Fork.Reused["first"] = legacy
	if err := validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("legacy success accepted")
	}
	f.snapshot.Fork.Reused["first"] = original
	f.snapshot.SourceRunID = uuid.NewString()
	if err := validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("same-project forged run accepted")
	}
	f.snapshot.SourceRunID = f.run
	f.snapshot.Workflow.Input = "changed"
	if err := validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
		t.Fatal("Prefix precondition bypassed")
	}
}

func TestExperienceForkRejectsCachedLegacyAndDirectReused(t *testing.T) {
	f := newExperienceForkFixture(t)
	f.execute(t, 0)
	f.execute(t, 1)
	runtime := f.target(t)
	if err := workspace.EnsureFork(f.ctx, runtime.runID, runtime.scope.ProjectID, f.ns, f.run, f.snapshot.Fork.Commit); err != nil {
		t.Fatal(err)
	}
	cached, err := workspace.ImportCheckpoints(f.ctx, runtime.runID, f.snapshot.Fork)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.ImportFork(f.ctx); err == nil {
		t.Fatal("cached old proof was reblessed")
	}
	if err = runtime.ValidateReused(f.ctx, cached); err == nil {
		t.Fatal("direct reused bypass")
	}
	if err = runtime.ValidateReused(f.ctx, f.snapshot.Fork.Reused); err == nil {
		t.Fatal("source physical proof accepted in target")
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var count int
	if err = conn.QueryRow(f.ctx, `SELECT count(*) FROM pangu.experience_artifacts WHERE namespace=$1 AND artifact->>'run_id'=$2`, f.ns, runtime.runID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("legacy cache signed after publication")
	}
}
