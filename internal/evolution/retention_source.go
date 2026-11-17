package evolution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func retentionLoadTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, generation int) (*intentRecord, experience.Policy, error) {
	var policy experience.Policy
	if err := lockScope(ctx, tx, scope); err != nil {
		return nil, policy, err
	}
	intent, err := readIntentTx(ctx, tx, scope, id)
	if err != nil {
		return nil, policy, err
	}
	if intent.Generation != generation {
		return nil, policy, ErrConflict
	}
	var data []byte
	var actor string
	err = tx.QueryRow(ctx, `SELECT policy,actor_id::text FROM pangu.experience_policies WHERE namespace=$1 AND project_id=$2 AND revision=$3`, scope.Namespace, scope.ProjectID, intent.PolicyRevision).Scan(&data, &actor)
	if err != nil {
		return nil, policy, err
	}
	if err = json.Unmarshal(data, &policy); err != nil {
		return nil, policy, err
	}
	if actor == "" || policy.ActorID != actor || policy.ProjectID != scope.ProjectID || policy.Revision != intent.PolicyRevision {
		return nil, policy, ErrForbidden
	}
	return intent, policy, nil
}

func retentionReasonTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy) (string, error) {
	reason := ""
	if policy.Observation.SourceRetentionAuthorized && policy.Observation.RetentionSeconds > 0 {
		var expired bool
		err := tx.QueryRow(ctx, `SELECT created_at+($2 * interval '1 second')<=clock_timestamp() FROM pangu.evolution_intents WHERE id=$1`, intent.ID, policy.Observation.RetentionSeconds).Scan(&expired)
		if err != nil {
			return "", err
		}
		if expired {
			reason = "retention_expired"
		}
	}
	for _, id := range intent.Config.Request.MemoryVersionIDs {
		current, err := retentionMemoryReason(ctx, tx, scope, id)
		if err != nil {
			return "", err
		}
		if current != "" {
			reason = current
		}
	}
	return reason, nil
}

func retentionMemoryReason(ctx context.Context, tx pgx.Tx, scope Scope, id string) (string, error) {
	var expired bool
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT expires_at<=clock_timestamp(),sources FROM pangu.experience_versions WHERE namespace=$1 AND project_id=$2 AND id=$3 FOR SHARE`, scope.Namespace, scope.ProjectID, id).Scan(&expired, &raw)
	if err != nil {
		return "", err
	}
	var sources []experience.Source
	if err = json.Unmarshal(raw, &sources); err != nil {
		return "", err
	}
	reason := ""
	if expired {
		reason = "retention_expired"
	}
	for _, source := range sources {
		current, err := retentionConsentReason(ctx, tx, scope, source)
		if err != nil {
			return "", err
		}
		if current != "" {
			reason = current
		}
	}
	return reason, nil
}

func retentionConsentReason(ctx context.Context, tx pgx.Tx, scope Scope, source experience.Source) (string, error) {
	if source.ProjectID != scope.ProjectID {
		return "", ErrForbidden
	}
	var revoked, expired bool
	err := tx.QueryRow(ctx, `SELECT revoked_at IS NOT NULL,expires_at<=clock_timestamp() FROM pangu.learning_source_consents WHERE namespace=$1 AND project_id=$2 AND source_id=$3 AND source_kind=$4 AND source_hash=$5 AND run_id=$6 AND node_id=$7 FOR SHARE`, scope.Namespace, scope.ProjectID, source.ID, source.Kind, source.Hash, source.RunID, source.NodeID).Scan(&revoked, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if revoked {
		return "source_withdrawn", nil
	}
	if expired {
		return "source_expired", nil
	}
	return "", nil
}
