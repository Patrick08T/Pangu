package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/jackc/pgx/v5"
)

func evolutionDispatchRequestTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan) (string, error) {
	var raw []byte
	var run, executionRun, status, hash, input string
	var created time.Time
	var started bool
	err := tx.QueryRow(ctx, `SELECT s.snapshot,s.payload_hash,s.input_hash,coalesce(s.run_id::text,''),s.created_at,e.status,coalesce(e.run_id::text,''),
 EXISTS(SELECT 1 FROM pangu.evolution_case_runtime r WHERE r.request_id=e.request_id) OR EXISTS(SELECT 1 FROM pangu.evolution_machine_results r WHERE r.request_id=e.request_id)
 FROM pangu.submissions s JOIN pangu.evolution_executions e ON e.request_id=s.id
 WHERE s.id=$1 AND s.namespace=$2 AND s.project_id=$3 AND s.created_by=$4 AND s.operation='evolution' AND s.purged_at IS NULL
 AND e.intent_id=$5 AND e.generation=$6 AND e.arm=$7 AND e.case_id=$8 AND e.repeat=$9
 AND e.input_hash=$10 AND e.max_calls=$11 AND e.max_tokens=$12 FOR SHARE OF s,e`, p.RequestID, s.Namespace, s.ProjectID, s.ActorID, p.Key.IntentID, p.Key.Generation, p.Key.Arm, p.Key.CaseID, p.Key.Repeat, p.InputHash, p.MaxCalls, p.MaxTokens).Scan(&raw, &hash, &input, &run, &created, &status, &executionRun, &started)
	if err != nil {
		return "", err
	}
	expected := evolutionSubmissionSnapshot(s, p)
	if hash != evolutionHash(expected) || input != evolutionTextHash(expected.Workflow.Input) || !evolutionDispatchSameJSON(raw, evolutionJSON(expected)) {
		return "", evolution.ErrConflict
	}
	if run != "" {
		if executionRun != "" && executionRun != run {
			return "", evolution.ErrConflict
		}
		return run, nil
	}
	if executionRun != "" || status != "reserved" || started {
		return "", evolution.ErrConflict
	}
	if time.Since(created) >= 89*24*time.Hour {
		return "", submission.ErrExpired
	}
	return "", nil
}

// UseNumber keeps numeric identities exact; comparing both complete objects also
// rejects unknown fields, case aliases and approval/fork markers hidden by decoding.
func evolutionDispatchSameJSON(a, b []byte) bool {
	canonical := func(raw []byte) ([]byte, error) {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	}
	left, err := canonical(a)
	if err != nil {
		return false
	}
	right, err := canonical(b)
	return err == nil && bytes.Equal(left, right)
}
