//go:build integration

package learning

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func (f learningFixture) canary(t *testing.T) experience.Version {
	t.Helper()
	version, intent, checker := f.claim(t)
	activation, err := CheckClaim(f.ctx, f.scope, version.ID, checker.ID, intent.ID, intent.Generation)
	if err != nil {
		t.Fatal(err)
	}
	version, err = f.store.Activate(f.ctx, f.scope, version.ID, activation)
	if err != nil {
		t.Fatal(err)
	}
	delivery := experience.DeliveryPolicy{Mode: "canary", TaskScope: []string{"business"}, TrafficBasisPoints: 10000}
	data, _ := json.Marshal(delivery)
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,2,'canary',$2,$3)`, version.ID, data, f.actor.ID)
	version, err = f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func (f learningFixture) observationRun(t *testing.T, version experience.Version, decision string) experience.Source {
	t.Helper()
	source := f.report(t)
	var request, node string
	tx := f.transaction(t)
	if err := tx.QueryRow(f.ctx, `SELECT s.id::text,r.node_run_id::text FROM pangu.submissions s JOIN pangu.research_reports r ON r.run_id=s.run_id WHERE r.id=$1`, source.ID).Scan(&request, &node); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Freeze(f.ctx, f.scope, request, 1); err != nil {
		t.Fatal(err)
	}
	usage, _, err := f.store.Select(f.ctx, f.scope, experience.SelectionInput{ProjectID: f.scope.ProjectID, RunID: request, NodeID: "research", Round: 1,
		Generation: 1, ApprovalRevision: 1, BoundInputHash: digest([]byte("actual input")), RoleID: "researcher", Text: "validated result JSON contract", CanaryScope: "business"})
	if err != nil || len(usage.References) != 1 {
		t.Fatal("missing actual memory selection", usage, err)
	}
	artifact := experience.Artifact{ID: node, ProjectID: f.scope.ProjectID, RunID: source.RunID, NodeID: "research", Kind: "node_output",
		Hash: digest([]byte("actual output")), InputHash: digest([]byte("actual effective input")), Generation: 1, ProofVersion: 1, References: usage.References}
	if err = f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
		t.Fatal(err)
	}
	feedback := f.feedback(t, source, decision)
	if err = GrantSource(f.ctx, f.scope, feedback, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE pangu.run_history SET finished_at=clock_timestamp() WHERE run_id=$1`, source.RunID)
	return feedback
}

func TestMemoryObservationReadsOriginalFeedbackAndFrozenConsumption(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	source := f.observationRun(t, version, "accepted")
	observation, err := ObserveMemory(f.ctx, f.scope, version.ID, source.RunID)
	if err != nil || !observation.Accepted || observation.Source != source || observation.UsageHash == "" {
		t.Fatal("real observation rejected", observation, err)
	}
	again, err := ObserveMemory(f.ctx, f.scope, version.ID, source.RunID)
	if err != nil || again.ID != observation.ID {
		t.Fatal("retry duplicated observation", again, err)
	}
	if err = WithdrawSource(f.ctx, f.scope, source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ObserveMemory(f.ctx, f.scope, version.ID, source.RunID); err == nil {
		t.Fatal("withdrawn observation remained valid")
	}
}
