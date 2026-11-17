package evaluation

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// EvaluateStoredExperiment consumes trusted saved evidence, never a client verdict.
// current must be reconstructed from live compiler, policy and environment state.
func EvaluateStoredExperiment(ctx context.Context, namespace, project, id string, current ExperimentSpec) (ExperimentDecision, error) {
	return experimentTransaction(ctx, func(tx pgx.Tx) (ExperimentDecision, error) {
		return EvaluateStoredExperimentTx(ctx, tx, namespace, project, id, current)
	})
}

// EvaluateStoredExperimentTx persists only recomputed evidence under caller locks.
func EvaluateStoredExperimentTx(ctx context.Context, tx pgx.Tx, namespace, project, id string, current ExperimentSpec) (ExperimentDecision, error) {
	s, e, err := readActiveExperiment(ctx, tx, namespace, id)
	if err != nil {
		return ExperimentDecision{}, err
	}
	if e.LogicalProjectID != project {
		return ExperimentDecision{}, ErrNotFound
	}
	receipts, err := storedReceipts(ctx, tx, e)
	if err != nil {
		return ExperimentDecision{}, err
	}
	d := EvaluateExperiment(s, e, receipts, current)
	receiptHash := contentHash(receipts)
	d.DecisionHash = contentHash(struct {
		Experiment, Receipts, Outcome, Category string
		Generation                              int
	}{e.Hash, receiptHash, d.Outcome, d.Category, e.Generation})
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evaluation_experiment_decisions(experiment_id,generation,experiment_hash,receipt_set_hash,decision_hash,outcome,category)
   VALUES($1,$2,$3,$4,$5,$6,$7)`, e.ID, e.Generation, e.Hash, receiptHash, d.DecisionHash, d.Outcome, d.Category)
	return d, err
}
