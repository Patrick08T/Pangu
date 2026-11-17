package evaluation

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestExperimentDBDecisionIsRecomputedFromStoredEvidence(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(context.Background(), ns, dbExperimentSpec(s))
	if err != nil {
		t.Fatal(err)
	}
	d, err := EvaluateStoredExperiment(context.Background(), ns, s.ProjectID, e.ID, e.ExperimentSpec)
	if err != nil || d.Outcome != "insufficient" || !validHash(d.DecisionHash) {
		t.Fatalf("empty evidence blessed: %+v %v", d, err)
	}
	for _, r := range pairedReceipts(t, s, e) {
		if err = RecordExperimentReceipt(context.Background(), ns, r); err != nil {
			t.Fatal(err)
		}
	}
	d, err = EvaluateStoredExperiment(context.Background(), ns, s.ProjectID, e.ID, e.ExperimentSpec)
	if err != nil || d.Outcome != "rejected" || d.Category != "primary_threshold" {
		t.Fatalf("single family falsely sufficient: %+v %v", d, err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var hash, outcome string
	err = conn.QueryRow(context.Background(), "SELECT decision_hash,outcome FROM pangu.evaluation_experiment_decisions WHERE experiment_id=$1 ORDER BY id DESC LIMIT 1", e.ID).Scan(&hash, &outcome)
	if err != nil || hash != d.DecisionHash || outcome != d.Outcome {
		t.Fatalf("decision not persisted: %s %s %v", hash, outcome, err)
	}
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.evaluation_experiment_decisions SET outcome='passed' WHERE experiment_id=$1", e.ID); err == nil {
		t.Fatal("stored verdict can be blessed by mutation")
	}
	current := e.ExperimentSpec
	current.PolicyRevision++
	d, err = EvaluateStoredExperiment(context.Background(), ns, s.ProjectID, e.ID, current)
	if err != nil || d.Category != "drift" {
		t.Fatalf("policy drift ignored: %+v %v", d, err)
	}
}
