package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func (f *experienceForkFixture) memory(t *testing.T) string {
	t.Helper()
	actor := identity.Current(f.ctx)
	rebuildExec(t, `UPDATE pangu.memberships SET role='manager' WHERE user_id=$1 AND project_id=$2`, actor.ID, f.source.scope.ProjectID)
	policy := experience.Policy{ReadEnabled: true, Observation: experience.ObservationPolicy{SourceRetentionAuthorized: true, RetentionSeconds: 86400}}
	if _, err := f.source.store.SavePolicy(f.ctx, f.source.scope, policy, 0); err != nil {
		t.Fatal(err)
	}
	source := f.retainedSource(t)
	v, err := f.source.store.Create(f.ctx, f.source.scope, experience.Version{ProjectID: f.source.scope.ProjectID, Revision: 1, ExpiresAt: time.Now().Add(time.Hour), Sources: []experience.Source{source},
		Content: experience.Content{Kind: "practice", Observation: "confirmed fixture", Practice: "retain valid evidence", Conditions: "fixture only", Counterexamples: "other projects", Applicability: experience.Applicability{Roles: []string{"fixture"}}}})
	if err != nil {
		t.Fatal(err)
	}
	// Fixture seed represents a previously independently activated and served version.
	rebuildExec(t, `UPDATE pangu.experience_states SET validity='active' WHERE version_id=$1`, v.ID)
	rebuildExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,2,'serving','{"mode":"serving","task_scope":["business"],"traffic_basis_points":10000}',$2)`, v.ID, actor.ID)
	v, err = f.source.store.Read(f.ctx, f.source.scope, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.memories, err = experiencePayload(experience.Usage{References: []experience.Reference{{VersionID: v.ID, BodyHash: v.BodyHash, DeliveryRevision: 2}}}, []experience.Version{v})
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func (f experienceForkFixture) retainedSource(t *testing.T) experience.Source {
	t.Helper()
	actor := identity.Current(f.ctx)
	report, node, consent := uuid.NewString(), uuid.NewString(), uuid.NewString()
	body := []byte("independently retained source fixture")
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	history, _ := json.Marshal(map[string]any{"id": f.run, "project_id": f.source.scope.ProjectID, "status": "succeeded", "stop_confirmed": true, "nodes": []any{map[string]any{"id": node, "node_id": "retained"}}})
	rebuildExec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,now(),$3)`, f.run, f.ns, history)
	rebuildExec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'retained',$4,'{}','[]')`, report, f.run, node, body)
	rebuildExec(t, `INSERT INTO pangu.learning_source_consents(id,namespace,project_id,source_id,source_kind,run_id,node_id,source_hash,actor_id,expires_at) VALUES($1,$2,$3,$4,'report',$5,'retained',$6,$7,now()+interval '1 hour')`, consent, f.ns, f.source.scope.ProjectID, report, f.run, hash, actor.ID)
	rebuildExec(t, `INSERT INTO pangu.learning_source_exposures(consent_id,namespace,project_id,source_id,run_id,task_id,input_hash,unresolved) SELECT $1,namespace,project_id,$2,run_id,task_id,encode(sha256(convert_to(snapshot->'workflow'->>'input','UTF8')),'hex'),false FROM pangu.submissions WHERE id=$3`, consent, report, f.previous.ID)
	return experience.Source{ID: report, Kind: "report", RunID: f.run, NodeID: "retained", ProjectID: f.source.scope.ProjectID, Hash: hash}
}

func TestExperienceForkPreservesMemoryDependencyAndRejectsInvalidation(t *testing.T) {
	for _, kind := range []string{"revoked", "expired", "sticky", "deleted", "source_withdrawn"} {
		t.Run(kind, func(t *testing.T) {
			f := newExperienceForkFixture(t)
			version := f.memory(t)
			f.execute(t, 0)
			f.execute(t, 1)
			runtime := f.target(t)
			if err := runtime.ValidateFork(f.ctx); err != nil {
				t.Fatal(err)
			}
			if err := workspace.EnsureFork(f.ctx, runtime.runID, runtime.scope.ProjectID, f.ns, f.run, f.snapshot.Fork.Commit); err != nil {
				t.Fatal(err)
			}
			imported, err := runtime.ImportFork(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			input := runtime.request.Snapshot.Workflow.NodeInput(runtime.request.Snapshot.Workflow.Nodes[2], imported)
			if err = runtime.Prepare(f.ctx, &input); err != nil {
				t.Fatal(err)
			}
			f.invalidateMemory(t, version, kind)
			if err = runtime.Prepare(f.ctx, &input); err == nil {
				t.Fatal("downstream lost source memory influence")
			}
			if err = runtime.ValidateReused(f.ctx, imported); err == nil {
				t.Fatal("direct reuse ignored invalidation")
			}
			if _, err = runtime.ImportFork(f.ctx); err == nil {
				t.Fatal("cached import ignored invalidation")
			}
			if err = validateForkExperience(f.ctx, f.ns, f.previous, &f.snapshot); err == nil {
				t.Fatal("pre-workspace source gate ignored invalidation")
			}
		})
	}
}

func (f experienceForkFixture) invalidateMemory(t *testing.T, version, kind string) {
	t.Helper()
	switch kind {
	case "revoked", "expired":
		rebuildExec(t, `UPDATE pangu.experience_states SET validity=$2 WHERE version_id=$1`, version, kind)
	case "deleted":
		rebuildExec(t, `DELETE FROM pangu.experience_bodies WHERE version_id=$1`, version)
	case "source_withdrawn":
		rebuildExec(t, `UPDATE pangu.learning_source_consents SET revoked_at=now() WHERE run_id=$1`, f.run)
	case "sticky":
		rebuildExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,3,'paused','{"mode":"paused","block_frozen":true}','test'),($1,4,'serving','{"mode":"serving","task_scope":["business"],"traffic_basis_points":10000}','test')`, version)
	}
}
