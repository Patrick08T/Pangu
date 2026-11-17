package learning

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SourceEvidence is read directly from the original source, never a retained copy.
type SourceEvidence struct {
	Source           experience.Source `json:"source"`
	Body             []byte            `json:"body"`
	TaskID           string            `json:"task_id"`
	EvidenceRevision int64             `json:"evidence_revision"`
	FinishedAt       time.Time         `json:"finished_at"`
}

func ReadSource(ctx context.Context, scope experience.Scope, source experience.Source) (SourceEvidence, error) {
	return transaction(ctx, scope, "source_read", func(tx pgx.Tx, scope experience.Scope) (SourceEvidence, error) {
		return sourceTx(ctx, tx, scope, source, true)
	})
}

// GrantSource is an interactive, project-manager retention opt-in for exact bytes.
func GrantSource(ctx context.Context, scope experience.Scope, source experience.Source, expiresAt time.Time) error {
	_, err := transaction(ctx, scope, "source_write", func(tx pgx.Tx, scope experience.Scope) (bool, error) {
		policy, err := policyTx(ctx, tx, scope)
		if err != nil {
			return false, err
		}
		if expiresAt.Before(time.Now()) || expiresAt.After(time.Now().Add(time.Duration(policy.Observation.RetentionSeconds)*time.Second)) {
			return false, experience.ErrInvalid
		}
		if _, err = sourceTx(ctx, tx, scope, source, false); err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pangu.learning_source_consents
		 (id,namespace,project_id,source_id,source_kind,run_id,node_id,source_hash,actor_id,expires_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(namespace,project_id,source_kind,source_id,source_hash) DO NOTHING`,
			uuid.NewString(), scope.Namespace, scope.ProjectID, source.ID, source.Kind, source.RunID, source.NodeID, source.Hash, scope.ActorID, expiresAt)
		if err != nil {
			return false, err
		}
		if err = sourceConsent(ctx, tx, scope, source); err != nil {
			return false, err
		}
		if err = recordExposure(ctx, tx, scope, source); err != nil {
			return false, err
		}
		return true, auditTx(ctx, tx, scope, source.ID, map[string]string{"source_hash": source.Hash})
	})
	return err
}

func WithdrawSource(ctx context.Context, scope experience.Scope, sourceID string) error {
	_, err := transaction(ctx, scope, "source_write", func(tx pgx.Tx, scope experience.Scope) (bool, error) {
		id, err := uuid.Parse(sourceID)
		if err != nil {
			return false, experience.ErrInvalid
		}
		sourceID = id.String()
		_, err = tx.Exec(ctx, `UPDATE pangu.learning_source_consents SET revoked_at=coalesce(revoked_at,now())
		 WHERE namespace=$1 AND project_id=$2 AND source_id=$3`, scope.Namespace, scope.ProjectID, sourceID)
		if err != nil {
			return false, err
		}
		if err = eraseWithdrawnSourceTx(ctx, tx, scope, sourceID); err != nil {
			return false, err
		}
		return true, auditTx(ctx, tx, scope, sourceID, struct{}{})
	})
	return err
}

func verifySource(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source) error {
	_, err := sourceTx(ctx, tx, scope, source, true)
	return err
}

func sourceConsent(ctx context.Context, tx pgx.Tx, scope experience.Scope, source experience.Source) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT c.id FROM pangu.learning_source_consents c JOIN pangu.users u ON u.id=c.actor_id
	 LEFT JOIN pangu.memberships m ON m.user_id=u.id AND m.project_id=c.project_id
	 WHERE c.namespace=$1 AND c.project_id=$2 AND c.source_id=$3 AND c.source_kind=$4 AND c.run_id=$5
	 AND c.node_id=$6 AND c.source_hash=$7 AND c.expires_at>clock_timestamp() AND c.revoked_at IS NULL
	 AND u.namespace=c.namespace AND NOT u.disabled AND (u.is_admin OR (m.enabled AND m.role='manager')) FOR SHARE OF c,u`,
		scope.Namespace, scope.ProjectID, source.ID, source.Kind, source.RunID, source.NodeID, source.Hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return experience.ErrForbidden
	}
	return err
}
