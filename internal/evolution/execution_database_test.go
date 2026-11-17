//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type executionEvidence struct{ result MachineResult }

func (e *executionEvidence) ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error) {
	return e.result, nil
}
func (e *executionEvidence) ReadObservationTx(context.Context, pgx.Tx, Scope, string) (BusinessEvidence, error) {
	return BusinessEvidence{}, ErrChecker
}

func preparedExecution(t *testing.T) (reservationFixture, *ExecutionPlan, *executionEvidence) {
	t.Helper()
	f := newReservationFixture(t)
	evidence := &executionEvidence{}
	f.service.Evidence = evidence
	intent, err := f.service.ReserveCandidate(context.Background(), f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	caseID := ""
	for _, c := range f.suite.Cases {
		if c.Partition != "development" {
			caseID = c.ID
			break
		}
	}
	plan, err := f.service.PrepareCase(context.Background(), f.scope, CaseKey{IntentID: intent.ID, Generation: 1, Arm: "candidate", CaseID: caseID, Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	return f, plan, evidence
}

func actualMachineFixture(plan *ExecutionPlan) MachineResult {
	arm := plan.Experiment.Candidate
	if plan.Key.Arm == "baseline" {
		arm = plan.Experiment.Baseline
	}
	return MachineResult{RequestID: plan.RequestID, RunID: uuid.NewString(), NodeID: "answer", Text: `{"answer":1}`, Status: "completed", Calls: 1, Tokens: 10, Cost: .001, Currency: "USD", CostMeasured: true, TokensMeasured: true, DurationMeasured: true, DurationSeconds: .1, Generation: plan.Key.Generation, ExperimentHash: plan.Experiment.Hash, ExecutionInputHash: plan.InputHash, InputTextHash: hashText(plan.Definition.Input), EnvironmentHash: plan.Environment.Hash, CompiledHash: arm.CompiledHash, RoleHash: arm.RoleHash, MemoryHash: arm.MemoryHash, SourceHash: hashText("real-ledger"), StopConfirmed: true}
}

func TestPrepareCasePersistsDispatchAndRechecksLivePolicy(t *testing.T) {
	f, plan, _ := preparedExecution(t)
	inputHash := ""
	for _, c := range f.suite.Cases {
		if c.ID == plan.Key.CaseID {
			inputHash = c.InputHash
		}
	}
	if plan.RequestID == "" || hashText(plan.Definition.Input) != inputHash || plan.Key.Arm != "candidate" || plan.MaxCalls <= 0 || plan.Environment.Currency != "USD" {
		t.Fatal("没有真实受控计划", plan)
	}
	retry, err := f.service.PrepareCase(context.Background(), f.scope, plan.Key)
	if err != nil || retry.RequestID != plan.RequestID {
		t.Fatal("重试重复执行预留", retry, err)
	}
	stale := plan.Key
	stale.Generation++
	if _, err = f.service.PrepareCase(context.Background(), f.scope, stale); !errors.Is(err, ErrConflict) {
		t.Fatal("过期generation仍执行", err)
	}
	f.policy.Revision++
	f.policy.Revoked = true
	f.savePolicy(t)
	if _, err = f.service.PrepareCase(context.Background(), f.scope, plan.Key); !errors.Is(err, ErrForbidden) {
		t.Fatal("撤权后重放仍执行", err)
	}
}

func TestRecordCaseResultRequiresActualExecutionIdentityAndMeasurements(t *testing.T) {
	f, plan, evidence := preparedExecution(t)
	ctx := context.Background()
	actual := actualMachineFixture(plan)
	changes := []func(*MachineResult){func(m *MachineResult) { m.InputTextHash = hashText("forged") }, func(m *MachineResult) { m.CompiledHash = hashText("forged") }, func(m *MachineResult) { m.MemoryHash = hashText("forged") }, func(m *MachineResult) { m.Generation++ }, func(m *MachineResult) { m.CostMeasured = false }, func(m *MachineResult) { m.StopConfirmed = false }}
	for _, change := range changes {
		evidence.result = actual
		change(&evidence.result)
		if err := f.service.RecordCaseResult(ctx, f.scope, plan.Key, plan.RequestID); err == nil {
			t.Fatal("伪造/未知实测被记录", evidence.result)
		}
	}
	evidence.result = actual
	if err := f.service.RecordCaseResult(ctx, f.scope, plan.Key, plan.RequestID); err != nil {
		t.Fatal(err)
	}
	if err := f.service.RecordCaseResult(ctx, f.scope, plan.Key, plan.RequestID); err != nil {
		t.Fatal("相同真实结果不幂等", err)
	}
	if _, err := f.service.PrepareCase(ctx, f.scope, plan.Key); !errors.Is(err, ErrConflict) {
		t.Fatal("已完成计划再次执行", err)
	}
}
