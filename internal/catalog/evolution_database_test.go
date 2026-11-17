package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type evolutionFixture struct {
	namespace, project, other, experiment string
	object                                *Object
	baseline                              *Version
	request                               AutomaticPublishRequest
}

func newEvolutionFixture(t *testing.T) evolutionFixture {
	t.Helper()
	ns, project, other := catalogDatabase(t)
	item, base := templateVersion(t, ns)
	payload := CandidatePayload{Agent: *base.Spec.Agent}
	payload.Agent.Instruction = "先验证权限，再检查项目并提供证据"
	prepared, err := PrepareExperimentCandidate(context.Background(), ns, project, base.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	experiment := seedCatalogExperiment(t, ns, project, base.ID, prepared)
	if err = StoreExperimentCandidate(context.Background(), ns, experiment, payload); err != nil {
		t.Fatal(err)
	}
	return evolutionFixture{ns, project, other, experiment, item, base, AutomaticPublishRequest{
		IntentID: uuid.NewString(), ExperimentID: experiment, Agent: PublishTarget{ObjectID: item.ID, ExpectedRevision: item.Revision, PublicBaseline: base.ID}}}
}

func seedCatalogExperiment(t *testing.T, ns, project, baseline string, p *PreparedCandidate) string {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	pool, suite, id := uuid.NewString(), uuid.NewString(), uuid.NewString()
	hash := contentHash(id)
	e := ExperimentIdentity{ID: id, Kind: "role_change", Purpose: "evaluation", LogicalProjectID: project,
		EnvironmentID: uuid.NewString(), EnvironmentHash: hash, PolicyRevision: 1, Generation: 1, BaselineVersion: baseline,
		CandidateHash: p.CandidateHash, SourceBaseline: hash, ConfigHash: hash,
		Baseline: ExperimentArm{p.BaselineHash, p.BaselineCompiledHash, hash}, Candidate: ExperimentArm{p.CandidateHash, p.CandidateCompiledHash, hash}}
	data, _ := json.Marshal(e)
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates) VALUES($1,$2,$3,$4,$4,1)`, []any{pool, ns, project, hash}},
		{`INSERT INTO pangu.evaluation_suites(id,namespace,project_id,pool_id,spec,spec_hash) VALUES($1,$2,$3,$4,'{}',$5)`, []any{suite, ns, project, pool, hash}},
		{`INSERT INTO pangu.evaluation_experiments(id,namespace,project_id,suite_id,generation,spec,spec_hash) VALUES($1,$2,$3,$4,1,$5,$6)`, []any{id, ns, project, suite, data, contentHash(e)}},
		{`INSERT INTO pangu.evaluation_experiment_decisions(experiment_id,generation,experiment_hash,receipt_set_hash,decision_hash,outcome,category) VALUES($1,1,$2,$3,$3,'passed','passed')`, []any{id, contentHash(e), hash}},
	} {
		if _, err = conn.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func catalogTestGuard(context.Context, pgx.Tx, ExperimentIdentity, AutomaticPublishRequest) error {
	return nil
}
func catalogStateGuard(context.Context, pgx.Tx, Version) error { return nil }

func TestCatalogEvolutionTrustedCompileAndProjectScope(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	d, identity, err := CompileExperiment(ctx, f.namespace, f.experiment, "candidate", "任务")
	if err != nil || identity.ID != f.experiment {
		t.Fatalf("trusted compile: %v", err)
	}
	if err = CheckDefinition(ctx, f.namespace, *d); err == nil {
		t.Fatal("普通提交接受了候选绕过")
	}
	out, err := PublishAutomatic(ctx, f.namespace, f.request, catalogTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []VersionScope{{}, {ProjectID: f.other}, {ProjectID: f.project}} {
		if _, err = ReadVersionScoped(ctx, f.namespace, out.Agent.ID, scope); !errors.Is(err, ErrNotFound) {
			t.Fatalf("private observing disclosed: %v", err)
		}
	}
	if _, err = ReadVersion(ctx, f.namespace, out.Agent.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("legacy read disclosed auto", err)
	}
	trial := VersionScope{ProjectID: f.project, AllowObserving: true}
	if _, err = CompileScoped(ctx, f.namespace, out.Agent.ID, trial, "任务"); err != nil {
		t.Fatal(err)
	}
	choices, err := AgentChoicesForProject(ctx, f.namespace, f.project)
	if err != nil || len(choices) != 1 || choices[0].Version.ID != f.baseline.ID {
		t.Fatal("observing became default", err)
	}
}

func TestCatalogEvolutionPublishIdempotentAndPreservesDraft(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	first, err := PublishAutomatic(ctx, f.namespace, f.request, catalogTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PublishAutomatic(ctx, f.namespace, f.request, catalogTestGuard)
	if err != nil || second.Agent.ID != first.Agent.ID {
		t.Fatal("publish is not idempotent", err)
	}
	current, err := Get(ctx, f.namespace, f.object.ID)
	if err != nil || current.Revision != f.object.Revision || !sameJSON(current.Spec, f.object.Spec) {
		t.Fatal("publish changed user draft", err)
	}
	versions, err := Versions(ctx, f.namespace, f.object.ID, 0)
	if err != nil || len(versions) != 1 {
		t.Fatal("unscoped list disclosed auto", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var publicBody []byte
	if err = conn.QueryRow(ctx, "SELECT spec FROM pangu.catalog_versions WHERE id=$1", first.Agent.ID).Scan(&publicBody); err != nil {
		t.Fatal(err)
	}
	if string(publicBody) != "{}" {
		t.Fatal("private body copied to immutable shared row")
	}
}

func TestCatalogEvolutionRejectsUnpublishedDraft(t *testing.T) {
	f := newEvolutionFixture(t)
	f.object.Spec.Agent.Instruction = "用户还未发布的草稿"
	saved, err := Save(context.Background(), f.namespace, f.object.ID, f.object.Revision, f.object.Name, f.object.Spec)
	if err != nil {
		t.Fatal(err)
	}
	f.request.Agent.ExpectedRevision = saved.Revision
	if _, err = PublishAutomatic(context.Background(), f.namespace, f.request, catalogTestGuard); !errors.Is(err, ErrConflict) {
		t.Fatalf("draft conflict accepted: %v", err)
	}
}
