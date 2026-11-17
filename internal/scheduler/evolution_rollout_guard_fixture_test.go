//go:build integration

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rolloutGuardReader struct{ result evolution.MachineResult }

func (r *rolloutGuardReader) ReadExecutionTx(context.Context, pgx.Tx, evolution.Scope, string) (evolution.MachineResult, error) {
	return r.result, nil
}
func (*rolloutGuardReader) ReadObservationTx(context.Context, pgx.Tx, evolution.Scope, string) (evolution.BusinessEvidence, error) {
	return evolution.BusinessEvidence{}, evolution.ErrChecker
}

func rolloutGuardFixture(t *testing.T) (evolutionFixture, *evolution.Intent, evolution.EntrySnapshot, string) {
	t.Helper()
	return rolloutGuardMemoryFixture(t, false)
}

func rolloutGuardMemoryFixture(t *testing.T, memory bool) (evolutionFixture, *evolution.Intent, evolution.EntrySnapshot, string) {
	t.Helper()
	f := newEvolutionFixture(t, "http://127.0.0.1:1")
	f.service = newEvolutionService()
	ctx := context.Background()
	f.suite = rolloutGuardSuite(t, f)
	cron := uuid.NewString()
	rolloutGuardConfigure(t, &f, cron, memory)
	if memory {
		version := evolutionBusinessMemoryVersion(t, f)
		f.request.MemoryVersionIDs = []string{version.ID}
	}
	reader := &rolloutGuardReader{}
	f.service.Evidence = reader
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal("reserve real rollout", err)
	}
	for _, c := range f.suite.Cases {
		if c.Partition == "development" {
			continue
		}
		for _, arm := range []string{"baseline", "candidate"} {
			p, err := f.service.PrepareCase(ctx, f.scope, evolution.CaseKey{IntentID: intent.ID, Generation: 1, CaseID: c.ID, Arm: arm, Repeat: 1})
			if err != nil {
				t.Fatal(err)
			}
			reader.result = rolloutGuardMeasurement(p)
			if arm == "baseline" && c.Partition == "holdout" {
				reader.result.Text = `{"answer":"wrong"}`
			}
			if err = f.service.RecordCaseResult(ctx, f.scope, p.Key, p.RequestID); err != nil {
				t.Fatal(err)
			}
		}
	}
	decision, err := f.service.Decide(ctx, f.scope, intent.ID, 1)
	if err != nil || decision.Outcome != "passed" {
		t.Fatal("real checker did not pass", decision, err)
	}
	intent, err = f.service.Promote(ctx, f.scope, intent.ID, 1)
	if err != nil || intent.Publication == nil {
		t.Fatal("real publication missing", intent, err)
	}
	expected, err := f.service.PrepareSwitch(ctx, f.scope, intent.ID, 1, "cron:"+cron, "adopt")
	if err != nil {
		t.Fatal(err)
	}
	return f, intent, *expected, intent.ID + ":1:cron:" + cron + ":adopt"
}

func rolloutGuardMeasurement(p *evolution.ExecutionPlan) evolution.MachineResult {
	a := p.Experiment.Candidate
	if p.Key.Arm == "baseline" {
		a = p.Experiment.Baseline
	}
	return evolution.MachineResult{RequestID: p.RequestID, RunID: uuid.NewString(), NodeID: "answer", Text: `{"answer":"checker-only-value"}`, Status: "completed", Calls: 2, Tokens: 10, Cost: .001, Currency: "USD", CostMeasured: true, TokensMeasured: true, DurationMeasured: true, DurationSeconds: .1, Generation: 1, ExperimentHash: p.Experiment.Hash, ExecutionInputHash: p.InputHash, InputTextHash: testEvolutionText(p.Definition.Input), EnvironmentHash: p.Environment.Hash, CompiledHash: a.CompiledHash, RoleHash: a.RoleHash, MemoryHash: a.MemoryHash, SourceHash: testEvolutionText("boundary-ledger"), StopConfirmed: true}
}

func rolloutGuardSuite(t *testing.T, f evolutionFixture) *evaluation.FrozenSuite {
	t.Helper()
	spec := f.suite.SuiteSpec
	spec.ID = uuid.NewString()
	spec.Cases = nil
	spec.Rules.Confidence = .51
	spec.Rules.MinFamilies = 12
	spec.Rules.MaxCandidates = 1
	spec.Rules.Metrics = []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1}, {Name: "cost", Lower: 0, Upper: 1, Tolerance: 1}, {Name: "duration", Lower: 0, Upper: 1, Tolerance: 1}}
	for i := 0; i < 14; i++ {
		spec.Cases = append(spec.Cases, rolloutGuardSource(t, f, i))
	}
	suite, err := evaluation.CreateExperimentSuite(context.Background(), f.scope.Namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func rolloutGuardSource(t *testing.T, f evolutionFixture, i int) evaluation.SuiteCase {
	t.Helper()
	ctx := context.Background()
	input := fmt.Sprintf("independent rollout case %d", i)
	expected := `{"answer":"checker-only-value"}`
	c, err := evaluation.CreateCase(ctx, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, evaluation.CaseInput{Name: input, Input: input, Expected: expected, Criteria: []string{"json_equal"}, RetainInput: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := submission.Prepare(ctx, f.scope.Namespace, "rollout-source", uuid.NewString(), submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{ProjectID: f.scope.ProjectID, Input: input}})
	if err != nil {
		t.Fatal(err)
	}
	run := uuid.NewString()
	if _, err = submission.Dispatch(ctx, f.scope.Namespace, s.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
		t.Fatal(err)
	}
	partition := "holdout"
	if i == 12 {
		partition = "protection"
	}
	if i == 13 {
		partition = "development"
	}
	return evaluation.SuiteCase{ID: c.ID, FamilyID: uuid.NewString(), TaskID: s.TaskID, SourceRunID: run, InputHash: testEvolutionText(input), ExpectedHash: testEvolutionText(expected), CriteriaHash: testEvolutionHash(c.Criteria), CheckerHash: evolution.CheckerHash(), Partition: partition, Critical: partition == "protection"}
}

func rolloutGuardConfigure(t *testing.T, f *evolutionFixture, cron string, memory bool) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var raw []byte
	var p experience.Policy
	if err = conn.QueryRow(ctx, `SELECT policy FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=1`, f.scope.Namespace, f.scope.ProjectID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	p.Revision = 2
	p.ReadEnabled = memory
	p.Entrypoints = append(p.Entrypoints, "cron:"+cron)
	p.Evaluation.SuiteVersion = f.suite.ID
	p.Evaluation.HoldoutSet = f.suite.HoldoutHash
	p.Evaluation.MinimumSamples = 12
	p.Evaluation.Confidence = .51
	p.Evaluation.MaxCandidates = 1
	protection := []evaluation.SuiteCase{}
	for _, c := range f.suite.Cases {
		if c.Partition == "protection" {
			protection = append(protection, c)
		}
	}
	p.Evaluation.ProtectionSet = testEvolutionHash(protection)
	one := 1.
	p.Evaluation.ResourceTolerance = &one
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.experience_policies(namespace,project_id,revision,actor_id,policy) VALUES($1,$2,2,$3,$4)`, f.scope.Namespace, f.scope.ProjectID, f.scope.ActorID, evolutionJSON(p)); err != nil {
		t.Fatal(err)
	}
	d, err := catalog.Compile(ctx, f.scope.Namespace, f.request.Payload.FlowVersion, f.scope.ProjectID, "input")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.schedules(id,namespace,project_id,name,expression,workflow,created_by,remote_synced) VALUES($1::uuid,$2,$3,$1::text,'0 * * * *',$4,$5,true)`, cron, f.scope.Namespace, f.scope.ProjectID, evolutionJSON(d), f.scope.ActorID); err != nil {
		t.Fatal(err)
	}
	f.request.PolicyRevision = 2
	f.request.SuiteID = f.suite.ID
	f.request.Budget = experience.Budget{Calls: 52, Tokens: 5200}
	f.request.Entries = append(f.request.Entries, evolution.EntryTarget{Kind: "cron", ID: cron})
}
