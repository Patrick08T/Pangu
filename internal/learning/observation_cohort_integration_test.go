//go:build integration

package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func (f learningFixture) historicalCanary(t *testing.T, version experience.Version) experience.Version {
	t.Helper()
	delivery := experience.DeliveryPolicy{Mode: "canary", TaskScope: []string{"business"}, TrafficBasisPoints: 10000}
	data, _ := json.Marshal(delivery)
	f.exec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id,created_at) VALUES($1,2,'canary',$2,$3,now()-interval '120 seconds')`, version.ID, data, f.actor.ID)
	version, err := f.store.Read(f.ctx, f.scope, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func (f learningFixture) historicalObservation(t *testing.T, version experience.Version, index int, omitArtifact ...bool) experience.Source {
	t.Helper()
	source := f.report(t)
	input := fmt.Sprintf("independent business observation %d", index)
	f.exec(t, `UPDATE pangu.submissions SET snapshot=jsonb_set(snapshot,'{workflow,input}',to_jsonb($2::text)),input_hash=$3 WHERE run_id=$1`, source.RunID, input, digest([]byte(input)))
	var request, node string
	tx := f.transaction(t)
	if err := tx.QueryRow(f.ctx, `SELECT s.id::text,r.node_run_id::text FROM pangu.submissions s JOIN pangu.research_reports r ON r.run_id=s.run_id WHERE r.id=$1`, source.ID).Scan(&request, &node); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	mark, err := f.store.Freeze(f.ctx, f.scope, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	refs := []experience.Reference{{VersionID: version.ID, BodyHash: version.BodyHash, DeliveryRevision: version.DeliveryRevision}}
	usage := experience.Usage{Input: experience.SelectionInput{ProjectID: f.scope.ProjectID, RunID: request, NodeID: "research", Round: 1, Generation: 1, WatermarkGeneration: 1,
		ApprovalRevision: 1, BoundInputHash: digest([]byte(input)), RoleID: "researcher", CanaryScope: "business"}, WatermarkHash: mark.Hash, References: refs, InjectionHash: jsonHash(refs), CreatedAt: time.Now().UTC().Add(-90 * time.Second)}
	data, _ := json.Marshal(usage)
	f.exec(t, `INSERT INTO pangu.experience_usage(namespace,project_id,identity_hash,usage) VALUES($1,$2,$3,$4)`, f.scope.Namespace, f.scope.ProjectID, digest([]byte(request)), data)
	artifact := experience.Artifact{ID: node, ProjectID: f.scope.ProjectID, RunID: source.RunID, NodeID: "research", Kind: "node_output", Hash: digest([]byte("output")), InputHash: digest([]byte(input)), Generation: 1, ProofVersion: 1, References: refs}
	if len(omitArtifact) == 0 || !omitArtifact[0] {
		if err = f.store.SaveArtifact(f.ctx, f.scope, artifact); err != nil {
			t.Fatal(err)
		}
	}
	feedback := f.feedback(t, source, "accepted")
	if err = GrantSource(f.ctx, f.scope, feedback, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE pangu.run_history SET finished_at=now()-interval '80 seconds' WHERE run_id=$1`, source.RunID)
	return feedback
}

func TestMemoryServingRequiresWholeRetainedIndependentCohort(t *testing.T) {
	f := newLearningFixture(t)
	version, experiment, _ := f.benefit(t)
	version = f.historicalCanary(t, version)
	store := NewStoreWithBenefit(func(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error) {
		return experiment.ExperimentSpec, nil
	})
	change := experience.DeliveryChange{ExpectedRevision: version.DeliveryRevision, EvidenceID: experiment.ID, Policy: experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"business"}, TrafficBasisPoints: 10000}}
	if _, err := store.SetDelivery(f.ctx, f.scope, version.ID, change); err == nil {
		t.Fatal("empty cohort served")
	}
	for i := 0; i < 4; i++ {
		f.historicalObservation(t, version, i)
	}
	served, err := store.SetDelivery(f.ctx, f.scope, version.ID, change)
	if err != nil || served.Delivery != "serving" {
		t.Fatal("valid complete cohort rejected", served, err)
	}
}
