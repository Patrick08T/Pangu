//go:build integration

package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5"
)

func TestCancelEvaluationFencesEmptyIntentAndClearsPrivateCandidate(t *testing.T) {
	f := newReservationFixture(t)
	ctx := context.Background()
	intent, err := f.service.ReserveCandidate(ctx, f.scope, f.request)
	if err != nil {
		t.Fatal(err)
	}
	f.policy.Revision++
	f.policy.Enabled = false
	f.policy.AutoPublish = false
	f.savePolicy(t)
	done, err := f.service.CancelEvaluation(ctx, f.scope, intent.ID, 1, "授权撤回")
	if err != nil || done.State != "insufficient" {
		t.Fatal("无执行的撤权意图未有限终结", done, err)
	}
	if _, err = evaluation.ReadExperiment(ctx, f.scope.Namespace, f.scope.ProjectID, intent.ExperimentID); err == nil {
		t.Fatal("旧实验代际仍活跃")
	}
	assertCancelledLedger(t, f, intent)
	retry, err := f.service.CancelEvaluation(ctx, f.scope, intent.ID, 1, "重复请求")
	if err != nil || retry.State != "insufficient" {
		t.Fatal("终态重试不是幂等", retry, err)
	}
}

func TestCancelEvaluationRetainsClaimsUntilRealStopProof(t *testing.T) {
	f, plan, evidence := preparedExecution(t)
	ctx := context.Background()
	f.service.Evidence = &missingExecutionEvidence{}
	pending, err := f.service.CancelEvaluation(ctx, f.scope, plan.Key.IntentID, 1, "停止实验")
	if err != nil || pending.State != "reconciling" {
		t.Fatal("未知运行未保留占用", pending, err)
	}
	err = transaction(ctx, func(tx pgx.Tx) error {
		intent, e := readIntentTx(ctx, tx, f.scope, plan.Key.IntentID)
		if e != nil {
			return e
		}
		return verifyClaimsTx(ctx, tx, f.scope, intent)
	})
	if err != nil {
		t.Fatal("停止未确认提前释放", err)
	}
	if _, err = f.service.PrepareCase(ctx, f.scope, plan.Key); err == nil {
		t.Fatal("取消后仍派新执行")
	}
	storageErr := errors.New("connection unavailable")
	f.service.Evidence = &missingExecutionEvidence{failure: storageErr}
	if _, err = f.service.CancelEvaluation(ctx, f.scope, plan.Key.IntentID, 1, "存储暂不可用"); !errors.Is(err, storageErr) {
		t.Fatal("未知存储错误被吞", err)
	}
	f.service.Evidence = evidence
	evidence.result = actualMachineFixture(plan)
	evidence.result.StopConfirmed = false
	pending, err = f.service.CancelEvaluation(ctx, f.scope, plan.Key.IntentID, 1, "仍停止中")
	if err != nil || pending.State != "reconciling" {
		t.Fatal(pending, err)
	}
	evidence.result.StopConfirmed = true
	done, err := f.service.CancelEvaluation(ctx, f.scope, plan.Key.IntentID, 1, "终止已确认")
	if err != nil || done.State != "insufficient" {
		t.Fatal("真实停止未终结", done, err)
	}
	assertCancelledLedger(t, f, &Intent{ID: plan.Key.IntentID, ExperimentID: plan.Experiment.ID})
}

func TestDecideNaturalNonPassingPairsTerminateWithoutRerun(t *testing.T) {
	f := newReservationFixture(t)
	f, intent := completeDecisionFixture(t, f)
	ctx := context.Background()
	decision, err := f.service.Decide(ctx, f.scope, intent.ID, 1)
	if err != nil || decision.Outcome == "passed" {
		t.Fatal("低统计证据不应通过", decision, err)
	}
	err = transaction(ctx, func(tx pgx.Tx) error {
		saved, e := readIntentTx(ctx, tx, f.scope, intent.ID)
		if e == nil && saved.State != "insufficient" {
			t.Fatal("完整失败pair仍反复evaluating", saved.State)
		}
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCancelledLedger(t, f, intent)
}

type missingExecutionEvidence struct {
	executionEvidence
	failure error
}

func (e missingExecutionEvidence) ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error) {
	if e.failure != nil {
		return MachineResult{}, e.failure
	}
	return MachineResult{}, pgx.ErrNoRows
}

func assertCancelledLedger(t *testing.T, f reservationFixture, intent *Intent) {
	t.Helper()
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var claims, private int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.evolution_target_claims WHERE intent_id=$1`, intent.ID).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL`, intent.ExperimentID).Scan(&private); err != nil {
		t.Fatal(err)
	}
	if claims != 0 || private != 0 {
		t.Fatal("取消未原子清理占用/私有候选", claims, private)
	}
}
