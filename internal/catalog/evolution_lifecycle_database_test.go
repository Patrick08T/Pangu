package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
)

func lifecycleTx(t *testing.T, action func(pgx.Tx) error) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = action(tx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogEvolutionLifecycleAndPrivateBodyTombstone(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	pub, err := PublishAutomatic(ctx, f.namespace, f.request, catalogTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleTx(t, func(tx pgx.Tx) error {
		return TransitionAutomaticTx(ctx, tx, f.namespace, f.project, pub.Agent.ID, 1, "stable", catalogStateGuard)
	})
	choices, err := AgentChoicesForProject(ctx, f.namespace, f.project)
	if err != nil || len(choices) != 1 || choices[0].Version.ID != pub.Agent.ID {
		t.Fatal("stable not selected", err)
	}
	shared, err := AgentChoices(ctx, f.namespace)
	if err != nil || len(shared) != 1 || shared[0].Version.ID != f.baseline.ID {
		t.Fatal("stable leaked to shared defaults", err)
	}
	lifecycleTx(t, func(tx pgx.Tx) error {
		if err := TransitionAutomaticTx(ctx, tx, f.namespace, f.project, pub.Agent.ID, 2, "revoked", catalogStateGuard); err != nil {
			return err
		}
		return ClearAutomaticVersionTx(ctx, tx, f.namespace, f.project, pub.Agent.ID, catalogStateGuard)
	})
	if _, err = CompileScoped(ctx, f.namespace, pub.Agent.ID, VersionScope{ProjectID: f.project, AllowObserving: true}, "任务"); !errors.Is(err, ErrNotFound) {
		t.Fatal("tombstone runnable", err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, `UPDATE pangu.catalog_version_bodies SET spec='{}',cleared_at=NULL WHERE version_id=$1`, pub.Agent.ID); err == nil {
		t.Fatal("cleared body restored")
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.catalog_versions SET spec='{}' WHERE id=$1`, pub.Agent.ID); err == nil {
		t.Fatal("version identity mutable")
	}
	var exists bool
	if err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.catalog_versions WHERE id=$1)", pub.Agent.ID).Scan(&exists); err != nil || !exists {
		t.Fatal("version identity removed", err)
	}
}

func TestCatalogEvolutionInvalidatedCandidateCannotReturn(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.evaluation_experiments SET status='invalidated',generation=generation+1 WHERE id=$1", f.experiment); err != nil {
		t.Fatal(err)
	}
	lifecycleTx(t, func(tx pgx.Tx) error {
		return ClearExperimentCandidateTx(ctx, tx, f.namespace, f.project, f.experiment, func(context.Context, pgx.Tx) error { return nil })
	})
	if _, _, err = CompileExperiment(ctx, f.namespace, f.experiment, "candidate", "任务"); !errors.Is(err, ErrNotFound) {
		t.Fatal("invalidated experiment runnable", err)
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.catalog_experiment_candidates SET payload='{}',cleared_at=NULL WHERE experiment_id=$1`, f.experiment); err == nil {
		t.Fatal("cleared candidate restored")
	}
}

func TestCatalogEvolutionPublishesRoleAndCompleteFlowAtomically(t *testing.T) {
	f := newEvolutionFixture(t)
	ctx := context.Background()
	flow := &Flow{Nodes: []Binding{{ID: "step", AgentVersion: f.baseline.ID, Task: "输出验证证据", ReadOnly: true}}}
	object, err := Create(ctx, f.namespace, NewObject{Kind: "workflow", Name: "自动升级流程", Spec: Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	base, err := Publish(ctx, f.namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	payload := CandidatePayload{Agent: *f.baseline.Spec.Agent, FlowVersion: base.ID, BindingIDs: []string{"step"}}
	payload.Agent.Instruction = "新的受限检查说明"
	p, err := PrepareExperimentCandidate(ctx, f.namespace, f.project, f.baseline.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	f.experiment = seedCatalogExperiment(t, f.namespace, f.project, f.baseline.ID, p)
	if err = StoreExperimentCandidate(ctx, f.namespace, f.experiment, payload); err != nil {
		t.Fatal(err)
	}
	f.request.ExperimentID = f.experiment
	f.request.Flow = &PublishTarget{ObjectID: object.ID, ExpectedRevision: object.Revision, PublicBaseline: base.ID}
	out, err := PublishAutomatic(ctx, f.namespace, f.request, catalogTestGuard)
	if err != nil {
		t.Fatal(err)
	}
	if out.Flow == nil || out.Flow.Spec.Flow.Nodes[0].AgentVersion != out.Agent.ID || out.Flow.Spec.Flow.Nodes[0].Task != flow.Nodes[0].Task {
		t.Fatal("full flow replacement missing")
	}
	d, err := CompileScoped(ctx, f.namespace, out.Flow.ID, VersionScope{ProjectID: f.project, AllowObserving: true}, "任务")
	if err != nil || !strings.Contains(d.Nodes[0].Instruction, payload.Agent.Instruction) || !strings.Contains(d.Nodes[0].Instruction, flow.Nodes[0].Task) {
		t.Fatal("actual role+task lost", err)
	}
	if _, err = Compile(ctx, f.namespace, out.Flow.ID, f.project, "任务"); !errors.Is(err, ErrNotFound) {
		t.Fatal("observing flow became ordinary runnable", err)
	}
}
