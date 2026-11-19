package evolution

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"slices"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func validateMachineResult(plan *ExecutionPlan, result MachineResult) error {
	arm := plan.Experiment.Baseline
	if plan.Key.Arm == "candidate" {
		arm = plan.Experiment.Candidate
	}
	if result.RequestID != plan.RequestID || result.Generation != plan.Key.Generation || result.ExperimentHash != plan.Experiment.Hash || result.ExecutionInputHash != plan.InputHash || result.InputTextHash != hashText(plan.Definition.Input) || result.EnvironmentHash != plan.Environment.Hash || result.CompiledHash != arm.CompiledHash || result.RoleHash != arm.RoleHash || result.MemoryHash != arm.MemoryHash {
		return ErrConflict
	}
	if _, err := uuid.Parse(result.RunID); err != nil {
		return ErrConflict
	}
	source, err := hex.DecodeString(result.SourceHash)
	if err != nil || len(source) != 32 {
		return ErrConflict
	}
	found := false
	for _, node := range plan.Definition.Nodes {
		if node.ID == result.NodeID {
			found = true
		}
	}
	if plan.Environment.Backend == ResearchReplayBackend {
		node, err := ReplayCollector(plan.Definition)
		found = err == nil && node.ID == result.NodeID
	}
	if !found || !result.StopConfirmed || !slices.Contains([]string{"completed", "failed", "timeout", "infrastructure_error"}, result.Status) {
		return ErrConflict
	}
	if result.Calls < 0 || result.Tokens < 0 || math.IsNaN(result.Cost) || math.IsInf(result.Cost, 0) || result.Cost < 0 || math.IsNaN(result.DurationSeconds) || math.IsInf(result.DurationSeconds, 0) || result.DurationSeconds < 0 {
		return ErrChecker
	}
	if result.Status == "completed" && (!result.CostMeasured || !result.TokensMeasured || !result.DurationMeasured || result.Currency != plan.Environment.Currency || result.Calls < 1) {
		return ErrChecker
	}
	return nil
}

func machineReceipt(plan *ExecutionPlan, actual *evaluation.Case, result MachineResult) (evaluation.ExperimentReceipt, error) {
	receipt := evaluation.ExperimentReceipt{ExperimentID: plan.Experiment.ID, ExperimentHash: plan.Experiment.Hash, Generation: plan.Key.Generation, Arm: plan.Key.Arm, CaseID: plan.Key.CaseID, Repeat: plan.Key.Repeat, Attempt: 1, ExecutionInputHash: plan.InputHash, EnvironmentHash: plan.Environment.Hash, CompiledHash: result.CompiledHash, CheckerHash: checkerHash(plan.Environment.Backend), EvidenceHash: hashJSON(result), Status: result.Status}
	if result.Calls > plan.MaxCalls || result.Tokens > plan.MaxTokens {
		receipt.Status = "failed"
	}
	if receipt.Status != "completed" {
		return receipt, nil
	}
	pass, structure, err := machineScore(plan.Environment, actual, result.Text)
	if err != nil {
		return receipt, err
	}
	receipt.GatesPassed = structure
	receipt.ProtectionPassed = pass
	acceptance := 0.0
	if pass {
		acceptance = 1
	}
	receipt.Metrics = map[string]float64{"acceptance": acceptance, "cost": result.Cost, "duration": result.DurationSeconds}
	return receipt, nil
}

func machineScore(env Environment, actual *evaluation.Case, text string) (bool, bool, error) {
	if env.Backend == ResearchReplayBackend {
		return checkReplayResult(actual.Expected, actual.Criteria, text)
	}
	pass, err := checkJSONResult(actual.Expected, actual.Criteria, text)
	_, structureErr := strictObject(text)
	return pass, structureErr == nil, err
}

func storeMachineResultTx(ctx context.Context, tx pgx.Tx, scope Scope, plan *ExecutionPlan, result MachineResult, receipt evaluation.ExperimentReceipt) error {
	hash := hashJSON(result)
	result.Text = "" // Answers remain only in the trusted run store, never duplicated into evolution.
	var previous string
	err := tx.QueryRow(ctx, `SELECT result_hash FROM pangu.evolution_machine_results WHERE request_id=$1`, plan.RequestID).Scan(&previous)
	if err == nil {
		if previous != hash {
			return ErrConflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err = evaluation.RecordExperimentReceiptTx(ctx, tx, scope.Namespace, receipt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO pangu.evolution_machine_results(request_id,run_id,node_id,result_hash,result) VALUES($1,$2,$3,$4,$5)`, plan.RequestID, result.RunID, result.NodeID, hash, jsonBytes(result)); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE pangu.evolution_executions SET run_id=$2,node_id=$3,status=$4 WHERE request_id=$1`, plan.RequestID, result.RunID, result.NodeID, receipt.Status)
	return err
}
