package evolution

import (
	"context"
	"encoding/hex"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type cancellationExecution struct {
	request, input string
	result         []byte
}

func (s Service) cancelExecutionsTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord) (bool, error) {
	rows, err := tx.Query(ctx, `SELECT e.request_id::text,e.input_hash,r.result FROM pangu.evolution_executions e LEFT JOIN pangu.evolution_machine_results r USING(request_id) WHERE e.intent_id=$1 AND e.generation=$2 ORDER BY e.request_id FOR UPDATE OF e`, intent.ID, intent.Generation)
	if err != nil {
		return false, err
	}
	executions := []cancellationExecution{}
	for rows.Next() {
		var e cancellationExecution
		if err = rows.Scan(&e.request, &e.input, &e.result); err != nil {
			rows.Close()
			return false, err
		}
		executions = append(executions, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	stopped := true
	for _, e := range executions {
		ok, err := s.cancelExecutionTx(ctx, tx, scope, intent, e)
		if err != nil {
			return false, err
		}
		if !ok {
			stopped = false
		}
	}
	return stopped, nil
}

func (s Service) cancelExecutionTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, e cancellationExecution) (bool, error) {
	if len(e.result) > 0 {
		return persistedRuntimeStop(intent, e)
	}
	if s.Evidence == nil {
		return false, nil
	}
	proof, err := s.readStopTx(ctx, tx, scope, e.request)
	if errors.Is(err, ErrChecker) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !validRuntimeStop(intent, e, proof) {
		return false, nil
	}
	err = storeRuntimeStopTx(ctx, tx, e, proof)
	return err == nil, err
}

func validStopProof(intent *intentRecord, e cancellationExecution, result MachineResult) bool {
	if result.RequestID != e.request || result.Generation != intent.Generation || result.ExperimentHash != intent.ExperimentHash || result.ExecutionInputHash != e.input || !result.StopConfirmed || result.NodeID == "" || !slices.Contains([]string{"completed", "failed", "timeout", "infrastructure_error", "cancelled"}, result.Status) {
		return false
	}
	if _, err := uuid.Parse(result.RunID); err != nil {
		return false
	}
	hash, err := hex.DecodeString(result.SourceHash)
	return err == nil && len(hash) == 32
}
