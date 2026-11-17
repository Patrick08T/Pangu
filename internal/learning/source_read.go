package learning

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type businessRun struct {
	TaskID, ActorID  string
	Summary          json.RawMessage
	FinishedAt       time.Time
	EvidenceRevision int64
}

func sourceTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source, consent bool) (SourceEvidence, error) {
	out := SourceEvidence{Source: source}
	if err := validateSource(scope, source); err != nil {
		return out, err
	}
	if _, err := sourcePolicyTx(ctx, tx, scope); err != nil {
		return out, err
	}
	run, err := businessRunTx(ctx, tx, scope, source.RunID)
	if err != nil {
		return out, err
	}
	out.TaskID, out.FinishedAt, out.EvidenceRevision = run.TaskID, run.FinishedAt, run.EvidenceRevision
	if consent {
		if err = sourceConsent(ctx, tx, scope, source); err != nil {
			return out, err
		}
		if err = requireExposure(ctx, tx, scope, source); err != nil {
			return out, err
		}
	}
	if source.Kind == "report" {
		out.Body, err = reportBody(ctx, tx, source, run)
	} else {
		out.Body, err = feedbackBody(ctx, tx, scope, source)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, experience.ErrInvalidated
	}
	if err != nil {
		return out, err
	}
	if digest(out.Body) != source.Hash {
		return out, experience.ErrInvalidated
	}
	return out, nil
}

func validateSource(scope experience.Scope, source experience.Source) error {
	if source.ProjectID != scope.ProjectID || !slices.Contains([]string{"report", "feedback"}, source.Kind) || !validHash(source.Hash) {
		return experience.ErrInvalid
	}
	if _, err := uuid.Parse(source.ID); err != nil {
		return experience.ErrInvalid
	}
	if _, err := uuid.Parse(source.RunID); err != nil {
		return experience.ErrInvalid
	}
	if source.Kind == "report" && source.NodeID == "" {
		return experience.ErrInvalid
	}
	if source.Kind == "feedback" && source.NodeID != "" {
		return experience.ErrInvalid
	}
	return nil
}

func businessRunTx(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string) (businessRun, error) {
	var out businessRun
	var snapshot []byte
	var operation string
	err := tx.QueryRow(ctx, `SELECT s.task_id,s.created_by,s.snapshot,s.operation,h.summary,h.finished_at,
	 1+coalesce((SELECT revision FROM pangu.feedback f WHERE f.run_id=s.run_id AND NOT f.payload_expired),0)
	 FROM pangu.submissions s JOIN pangu.run_history h ON h.run_id=s.run_id AND h.namespace=s.namespace
	 JOIN pangu.workspace_runs w ON w.run_id=s.run_id AND w.project_id=s.project_id
	 WHERE s.namespace=$1 AND s.project_id=$2 AND s.run_id=$3 AND s.purged_at IS NULL
	 AND w.expired_at IS NULL AND w.purged_at IS NULL FOR SHARE OF s,h,w`, scope.Namespace, scope.ProjectID, id).
		Scan(&out.TaskID, &out.ActorID, &snapshot, &operation, &out.Summary, &out.FinishedAt, &out.EvidenceRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, experience.ErrInvalidated
	}
	if err != nil {
		return out, err
	}
	if !validBusinessRun(snapshot, out.Summary, operation, scope.ProjectID) {
		return out, experience.ErrForbidden
	}
	if err = disallowHiddenRun(ctx, tx, scope, id, out.TaskID, frozenInputHash(snapshot)); err != nil {
		return out, err
	}
	return out, nil
}

func validBusinessRun(snapshot, summary []byte, operation, project string) bool {
	var frozen struct {
		Purpose  string `json:"purpose"`
		Workflow struct {
			ProjectID string `json:"project_id"`
			Input     string `json:"input"`
		} `json:"workflow"`
	}
	var run struct {
		ProjectID     string `json:"project_id"`
		Status        string `json:"status"`
		StopConfirmed bool   `json:"stop_confirmed"`
	}
	if json.Unmarshal(snapshot, &frozen) != nil || json.Unmarshal(summary, &run) != nil {
		return false
	}
	return (frozen.Purpose == "" || frozen.Purpose == "business") && frozen.Workflow.ProjectID == project && frozen.Workflow.Input != "" &&
		run.ProjectID == project && run.StopConfirmed && slices.Contains([]string{"succeeded", "failed", "cancelled"}, run.Status) &&
		slices.Contains([]string{"submit", "fork", "rerun", "plan", "rebuild", "cron", "github", "retry", "continue", "repair"}, operation)
}

func reportBody(ctx context.Context, tx pgx.Tx, source experience.Source, run businessRun) ([]byte, error) {
	var body []byte
	err := tx.QueryRow(ctx, `SELECT r.body FROM pangu.research_reports r WHERE r.id=$1 AND r.run_id=$2
	 AND EXISTS(SELECT 1 FROM jsonb_array_elements($3::jsonb->'nodes') n WHERE n->>'id'=r.node_run_id::text AND n->>'node_id'=$4)
	 FOR SHARE OF r`, source.ID, source.RunID, []byte(run.Summary), source.NodeID).Scan(&body)
	return body, err
}

func disallowHiddenRun(ctx context.Context, tx pgx.Tx, scope experience.Scope, run, task, inputHash string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evaluation_identity_allocations
	 WHERE namespace=$1 AND project_id=$2 AND partition IN ('protection','holdout') AND
	 ((identity_kind='source' AND identity_key=$3) OR (identity_kind='task' AND identity_key=$4)))
	 OR EXISTS(SELECT 1 FROM pangu.evaluation_case_allocations WHERE namespace=$1 AND project_id=$2
	 AND partition IN ('protection','holdout') AND input_hash=$5)`,
		scope.Namespace, scope.ProjectID, run, task, inputHash).Scan(&blocked)
	if err == nil && blocked {
		return experience.ErrForbidden
	}
	return err
}
