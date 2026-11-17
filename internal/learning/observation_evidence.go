package learning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func deriveObservation(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy, runID string) (MemoryObservation, error) {
	out := MemoryObservation{ID: uuid.NewString(), VersionID: version.ID, DeliveryRevision: version.DeliveryRevision, PolicyRevision: policy.Revision, RunID: runID}
	if _, err := uuid.Parse(runID); err != nil {
		return out, experience.ErrInvalid
	}
	if version.Validity != "active" || (version.Delivery != "canary" && version.Delivery != "serving") || policy.Evaluation.PrimaryMetric != "acceptance" {
		return out, experience.ErrForbidden
	}
	run, err := businessRunTx(ctx, tx, scope, runID)
	if err != nil {
		return out, err
	}
	out.TaskID, out.FinishedAt = run.TaskID, run.FinishedAt
	out.UsageHash, err = observationUsage(ctx, tx, scope, version, policy, runID, run)
	if err != nil {
		return out, err
	}
	source := experience.Source{ProjectID: scope.ProjectID, RunID: runID, Kind: "feedback"}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM pangu.feedback WHERE namespace=$1 AND project_id=$2 AND run_id=$3 AND NOT payload_expired`, scope.Namespace, scope.ProjectID, runID).Scan(&source.ID); err != nil {
		return out, err
	}
	body, err := feedbackBody(ctx, tx, scope, source)
	if err != nil {
		return out, err
	}
	source.Hash = digest(body)
	if _, err = sourceTx(ctx, tx, scope, source, true); err != nil {
		if memoryGuardFailure(err) {
			return out, errors.Join(err, errObservationSource)
		}
		return out, err
	}
	var feedback FeedbackPayload
	var summary struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(body, &feedback) != nil || json.Unmarshal(run.Summary, &summary) != nil || feedback.TaskID != run.TaskID {
		return out, experience.ErrInvalidated
	}
	out.Source = source
	out.Accepted = summary.Status == "succeeded" && feedback.Decision == "accepted" && feedback.FirstDecision == "accepted"
	return out, nil
}

func persistObservation(ctx context.Context, tx pgx.Tx, scope experience.Scope, out MemoryObservation) (MemoryObservation, error) {
	source, _ := json.Marshal(out.Source)
	_, err := tx.Exec(ctx, `INSERT INTO pangu.learning_memory_observations(id,namespace,project_id,version_id,delivery_revision,policy_revision,run_id,task_id,usage_hash,source,accepted,finished_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(version_id,delivery_revision,task_id) DO NOTHING`, out.ID, scope.Namespace, scope.ProjectID, out.VersionID, out.DeliveryRevision, out.PolicyRevision, out.RunID, out.TaskID, out.UsageHash, source, out.Accepted, out.FinishedAt)
	if err != nil {
		return out, err
	}
	var stored MemoryObservation
	var data []byte
	err = tx.QueryRow(ctx, `SELECT id::text,version_id::text,delivery_revision,policy_revision,run_id::text,task_id::text,usage_hash,source,accepted,finished_at
 FROM pangu.learning_memory_observations WHERE namespace=$1 AND project_id=$2 AND version_id=$3 AND delivery_revision=$4 AND task_id=$5`, scope.Namespace, scope.ProjectID, out.VersionID, out.DeliveryRevision, out.TaskID).
		Scan(&stored.ID, &stored.VersionID, &stored.DeliveryRevision, &stored.PolicyRevision, &stored.RunID, &stored.TaskID, &stored.UsageHash, &data, &stored.Accepted, &stored.FinishedAt)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &stored.Source); err != nil {
		return out, err
	}
	out.ID = stored.ID
	if jsonHash(out) != jsonHash(stored) {
		return out, experience.ErrConflict
	}
	return stored, nil
}
