package experience

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErasureResult counts erased payload rows, never deleted immutable identities.
type ErasureResult struct {
	Versions int64 `json:"versions"`
	Bodies   int64 `json:"bodies"`
}

// EraseVersionsTx is a trusted lifecycle helper, not a user-authorized endpoint.
// The caller must hold the namespace identity lock and prove source withdrawal or
// retention expiry before calling. This helper locks the project, verifies every
// ID belongs to it, preserves existing invalid reasons, and never commits the tx.
func EraseVersionsTx(ctx context.Context, tx pgx.Tx, namespace, project string, ids []string, reason string) (ErasureResult, error) {
	out := ErasureResult{}
	validity := "expired"
	if reason == "source_withdrawn" {
		validity = "revoked"
	} else if reason != "source_expired" && reason != "retention_expired" {
		return out, ErrInvalid
	}
	if len(ids) < 1 || len(ids) > 1000 {
		return out, ErrInvalid
	}
	unique := map[string]bool{}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil || unique[id] {
			return out, ErrInvalid
		}
		unique[id] = true
	}
	if err := retentionScopeTx(ctx, tx, namespace, project); err != nil {
		return out, err
	}
	var count int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pangu.experience_versions v JOIN pangu.experience_states s ON s.version_id=v.id
  WHERE v.namespace=$1 AND v.project_id=$2 AND v.id=ANY($3::uuid[])`, namespace, project, ids).Scan(&count)
	if err != nil {
		return out, err
	}
	if count != len(ids) {
		return out, ErrForbidden
	}
	tag, err := tx.Exec(ctx, `UPDATE pangu.experience_states s SET
  validity=CASE WHEN s.validity IN ('revoked','expired') THEN s.validity ELSE $4 END,
  tombstoned=true,invalid_reason=CASE WHEN s.invalid_reason<>'' THEN s.invalid_reason ELSE $5 END,updated_at=now()
  FROM pangu.experience_versions v WHERE s.version_id=v.id AND v.namespace=$1 AND v.project_id=$2 AND v.id=ANY($3::uuid[])`, namespace, project, ids, validity, reason)
	if err != nil {
		return out, err
	}
	out.Versions = tag.RowsAffected()
	if out.Versions != int64(len(ids)) {
		return ErasureResult{}, ErrInvalidated
	}
	tag, err = tx.Exec(ctx, `DELETE FROM pangu.experience_bodies b USING pangu.experience_versions v
  WHERE b.version_id=v.id AND v.namespace=$1 AND v.project_id=$2 AND v.id=ANY($3::uuid[])`, namespace, project, ids)
	if err != nil {
		return ErasureResult{}, err
	}
	out.Bodies = tag.RowsAffected()
	return out, nil
}

func retentionScopeTx(ctx context.Context, tx pgx.Tx, namespace, project string) error {
	if _, err := uuid.Parse(project); err != nil || tx == nil || namespace == "" {
		return ErrInvalid
	}
	if err := Lock(ctx, tx, namespace, project); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.projects WHERE namespace=$1 AND id=$2)`, namespace, project).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrForbidden
	}
	return nil
}
