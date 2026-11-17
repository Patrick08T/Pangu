package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func invalidationDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func newExperienceInvalidationMemoryFixture(t *testing.T) experienceInvalidationFixture {
	t.Helper()
	f := newExperienceInvalidationFixture(t, func(f *experienceInvalidationFixture, d *workflow.Definition) {
		f.actor = uuid.NewString()
		rebuildExec(t, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1::uuid,$2,'test','test',$1::text,true)`, f.actor, f.ns)
		f.ctx = identity.WithActor(f.ctx, identity.Actor{ID: f.actor, Namespace: f.ns})
		role, version := uuid.NewString(), uuid.NewString()
		spec, _ := json.Marshal(map[string]any{"agent": d.Nodes[0]})
		rebuildExec(t, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1,$2,'agent','memory planner',$3)`, role, f.ns, spec)
		rebuildExec(t, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec) VALUES($1,$2,1,1,$3)`, version, role, spec)
		d.Nodes[0].AgentVersion = version
		policy := experience.Policy{Enabled: true, ReadEnabled: true, Roles: []string{role}, Delivery: experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"submit"}, TrafficBasisPoints: 10000}, Observation: experience.ObservationPolicy{RetentionSeconds: 86400, SourceRetentionAuthorized: true}}
		raw, _ := json.Marshal(policy)
		rebuildExec(t, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,1,$3,$4)`, f.ns, f.project, f.actor, raw)
		source := invalidationSource(t, f)
		scope := experience.Scope{Namespace: f.ns, ProjectID: f.project, ActorID: f.actor}
		if err := learning.GrantSource(f.ctx, scope, source, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		seedInvalidationVersion(t, f, role, source)
	})
	artifact, err := f.runtime.store.ReadArtifact(f.ctx, f.runtime.scope, f.parent.RunID)
	if err != nil || len(artifact.References) != 1 || artifact.References[0].VersionID != f.version {
		t.Fatalf("fixture did not inject actual experience: %+v %v", artifact.References, err)
	}
	return f
}

func invalidationSource(t *testing.T, f *experienceInvalidationFixture) experience.Source {
	t.Helper()
	run, task, node, id := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	snapshot, _ := json.Marshal(map[string]any{"workflow": map[string]any{"project_id": f.project, "input": "business evidence"}, "created_by": f.actor, "purpose": "business"})
	rebuildExec(t, `INSERT INTO pangu.submissions(id,namespace,project_id,operation,key_hash,payload_hash,snapshot,run_id,created_by) VALUES($1,$2,$3,'submit',$4,$4,$5,$6,$7)`, task, f.ns, f.project, task, snapshot, run, f.actor)
	rebuildExec(t, `INSERT INTO pangu.workspace_runs(run_id,project_id,base_commit,head_commit,worker_id,directory,ready) VALUES($1,$2,'base','head','test','unused',true)`, run, f.project)
	summary, _ := json.Marshal(map[string]any{"id": run, "project_id": f.project, "status": "succeeded", "stop_confirmed": true, "nodes": []any{map[string]any{"id": node, "node_id": "research"}}})
	rebuildExec(t, `INSERT INTO pangu.run_history(run_id,namespace,finished_at,summary) VALUES($1,$2,now(),$3)`, run, f.ns, summary)
	body := []byte(`{"result":{"valid":true}}`)
	rebuildExec(t, `INSERT INTO pangu.research_reports(id,run_id,node_run_id,title,body,output,evidence) VALUES($1,$2,$3,'contract',$4,'{}','[]')`, id, run, node, body)
	f.source = id
	return experience.Source{ID: id, Kind: "report", RunID: run, NodeID: "research", ProjectID: f.project, Hash: invalidationDigest(body)}
}

func seedInvalidationVersion(t *testing.T, f *experienceInvalidationFixture, role string, source experience.Source) {
	t.Helper()
	f.version = uuid.NewString()
	content := experience.Content{Kind: "procedure", Observation: "task has evidence", Practice: "check evidence", Conditions: "same task", Counterexamples: "unrelated input", Applicability: experience.Applicability{Roles: []string{role}, Keywords: []string{"task"}}}
	raw, _ := json.Marshal(content)
	sources, _ := json.Marshal([]experience.Source{source})
	delivery, _ := json.Marshal(experience.DeliveryPolicy{Mode: "serving", TaskScope: []string{"submit"}, TrafficBasisPoints: 10000})
	rebuildExec(t, `INSERT INTO pangu.experience_versions(id,namespace,project_id,entry_id,revision,body_hash,sources,expires_at,created_by) VALUES($1,$2,$3,$4,1,$5,$6,now()+interval '1 hour',$7)`, f.version, f.ns, f.project, uuid.NewString(), invalidationDigest(raw), sources, f.actor)
	rebuildExec(t, `INSERT INTO pangu.experience_bodies(version_id,content) VALUES($1,$2)`, f.version, raw)
	rebuildExec(t, `INSERT INTO pangu.experience_states(version_id,validity) VALUES($1,'active')`, f.version)
	rebuildExec(t, `INSERT INTO pangu.experience_deliveries(version_id,revision,mode,policy,actor_id) VALUES($1,1,'serving',$2,$3)`, f.version, delivery, f.actor)
}
