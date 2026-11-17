//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type evolutionConsumersFixture struct {
	evolutionFixture
	target evolution.RetentionTarget
}

func newEvolutionConsumersFixture(t *testing.T) evolutionConsumersFixture {
	t.Helper()
	base := newEvolutionFixture(t, "http://127.0.0.1:1")
	intent, err := base.service.ReserveCandidate(context.Background(), base.scope, base.request)
	if err != nil {
		t.Fatal(err)
	}
	f := evolutionConsumersFixture{evolutionFixture: base, target: evolution.RetentionTarget{IntentID: intent.ID, Generation: intent.Generation, ExperimentID: intent.ExperimentID}}
	version := f.privateVersion(t, base.request.BaselineVersion, "")
	f.target.VersionIDs = []string{version}
	f.exec(t, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, intent.ID, evolutionJSON(evolution.PublishedVersions{AgentID: version}))
	return f
}

func (f evolutionConsumersFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f evolutionConsumersFixture) privateVersion(t *testing.T, baseline, previous string) string {
	t.Helper()
	id, object := uuid.NewString(), uuid.NewString()
	f.exec(t, `INSERT INTO pangu.catalog_objects(id,namespace,kind,name,spec) VALUES($1::uuid,$2,'agent',$1::text,'{}')`, object, f.scope.Namespace)
	f.exec(t, `INSERT INTO pangu.catalog_versions(id,object_id,number,source_revision,spec,source_kind,source_experiment,project_id,public_baseline,project_previous,content_hash) VALUES($1,$2,1,NULL,'{}','auto',$3,$4,$5,NULLIF($6,'')::uuid,$7)`, id, object, f.target.ExperimentID, f.scope.ProjectID, baseline, previous, testEvolutionText(id))
	f.exec(t, `INSERT INTO pangu.catalog_version_state(version_id,state) VALUES($1,'revoked')`, id)
	f.exec(t, `INSERT INTO pangu.catalog_version_bodies(version_id,spec) VALUES($1,$2)`, id, evolutionJSON(map[string]any{"agent": f.request.Payload.Agent}))
	return id
}

func (f evolutionConsumersFixture) submit(t *testing.T, version string, node, pending bool) *submission.Record {
	t.Helper()
	def := workflow.Definition{ProjectID: f.scope.ProjectID, VersionID: version, Input: "business", Nodes: []workflow.Node{{ID: "work", Executor: "eino", Instruction: "bounded task"}}}
	if node {
		def.VersionID = ""
		def.Nodes[0].AgentVersion = version
	}
	record, err := submission.Prepare(context.Background(), f.scope.Namespace, "consumer-test", uuid.NewString(), submission.Snapshot{CreatedBy: f.scope.ActorID, Workflow: def})
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		return record
	}
	record, err = submission.Dispatch(context.Background(), f.scope.Namespace, record.ID, func(context.Context, *submission.Record) (string, error) { return uuid.NewString(), nil })
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func (f evolutionConsumersFixture) sameNamespaceProject(t *testing.T) evolutionConsumersFixture {
	t.Helper()
	other := f
	other.scope.ProjectID = uuid.NewString()
	f.exec(t, `INSERT INTO pangu.projects(id,namespace,name,source,branch,worker_id,vcs) VALUES($1,$2,'foreign project','unused','main','test','git')`, other.scope.ProjectID, f.scope.Namespace)
	return other
}

func (f evolutionConsumersFixture) stop(t *testing.T, run string) {
	t.Helper()
	if err := execution.RequestCancel(context.Background(), f.scope.Namespace, run); err != nil {
		t.Fatal(err)
	}
	// Controlled fixture for the existing remote-stop receipt, never a fabricated score.
	if err := execution.Reconciled(context.Background(), run); err != nil {
		t.Fatal(err)
	}
}

func (f evolutionConsumersFixture) guard(t *testing.T, phase string) (bool, error) {
	t.Helper()
	var allowed bool
	err := evolutionTransaction(context.Background(), func(tx pgx.Tx) error {
		var err error
		allowed, err = EvolutionRetentionGuard(context.Background(), tx, f.scope, f.target, phase)
		return err
	})
	return allowed, err
}
