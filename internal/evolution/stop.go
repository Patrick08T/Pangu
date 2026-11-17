package evolution

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type stopLedger struct {
	Proof StopProof `json:"stop_proof"`
}

func (s Service) readStopTx(ctx context.Context, tx pgx.Tx, scope Scope, request string) (StopProof, error) {
	if reader, ok := s.Evidence.(StopEvidenceReader); ok {
		return reader.ReadStopTx(ctx, tx, scope, request)
	}
	result, err := s.Evidence.ReadExecutionTx(ctx, tx, scope, request)
	if err != nil {
		return StopProof{}, err
	}
	// Compatibility for trusted readers that still return complete evidence; never fabricate a scoring receipt.
	return StopProof{RequestID: result.RequestID, RunID: result.RunID, Status: result.Status, Generation: result.Generation, ExperimentHash: result.ExperimentHash, ExecutionInputHash: result.ExecutionInputHash, SourceHash: result.SourceHash, StopConfirmed: result.StopConfirmed && result.NodeID != ""}, nil
}

func validRuntimeStop(intent *intentRecord, e cancellationExecution, proof StopProof) bool {
	if proof.RequestID != e.request || proof.Generation != intent.Generation || proof.ExperimentHash != intent.ExperimentHash || proof.ExecutionInputHash != e.input || !proof.StopConfirmed || !slices.Contains([]string{"completed", "failed", "timeout", "infrastructure_error", "cancelled"}, proof.Status) {
		return false
	}
	if _, err := uuid.Parse(proof.RunID); err != nil {
		return false
	}
	hash, err := hex.DecodeString(proof.SourceHash)
	return err == nil && len(hash) == 32
}

func persistedRuntimeStop(intent *intentRecord, e cancellationExecution) (bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(e.result, &envelope); err != nil {
		return false, err
	}
	if raw, ok := envelope["stop_proof"]; ok {
		var proof StopProof
		if err := json.Unmarshal(raw, &proof); err != nil {
			return false, err
		}
		return len(envelope) == 1 && validRuntimeStop(intent, e, proof), nil
	}
	var result MachineResult
	if err := json.Unmarshal(e.result, &result); err != nil {
		return false, err
	}
	return validStopProof(intent, e, result), nil
}

func storeRuntimeStopTx(ctx context.Context, tx pgx.Tx, e cancellationExecution, proof StopProof) error {
	ledger := stopLedger{Proof: proof}
	// node_id is ledger metadata only. The JSON body cannot decode into a valid MachineResult.
	if _, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_machine_results(request_id,run_id,node_id,result_hash,result) VALUES($1,$2,'$runtime-stop',$3,$4)`, e.request, proof.RunID, hashJSON(ledger), jsonBytes(ledger)); err != nil {
		return err
	}
	status := proof.Status
	if status == "cancelled" {
		status = "failed"
	}
	_, err := tx.Exec(ctx, `UPDATE pangu.evolution_executions SET status=$2,run_id=$3,node_id='$runtime-stop' WHERE request_id=$1`, e.request, status, proof.RunID)
	return err
}
