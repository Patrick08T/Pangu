package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Expressions are fixed internal columns, never SQL supplied by a caller.
func experienceErasureUUIDSQL(field string) string {
	const plain = `[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}`
	const pattern = `^((urn:uuid:)?` + plain + `|[{]` + plain + `[}]|[0-9a-f]{32})$`
	return `(CASE WHEN ` + field + ` ~* '` + pattern + `' THEN replace(lower(` + field + `),'urn:uuid:','')::uuid END)`
}

var experienceErasureConsentSQL = `EXISTS(SELECT 1 FROM jsonb_array_elements(v.sources) src
 JOIN pangu.learning_source_consents c ON c.namespace=v.namespace AND c.project_id=v.project_id
 AND c.source_id=` + experienceErasureUUIDSQL("src->>'id'") + ` AND c.source_kind=src->>'kind' AND c.source_hash=src->>'hash'
 AND c.run_id=` + experienceErasureUUIDSQL("src->>'run_id'") + ` AND c.node_id=src->>'node_id'
 WHERE ` + experienceErasureUUIDSQL("src->>'project_id'") + `=v.project_id
 AND (c.revoked_at IS NOT NULL OR c.expires_at<=clock_timestamp()))`

// A mutable invalid_reason, policy pause or missing body alone is never authority.
func experienceErasureRootTx(ctx context.Context, tx pgx.Tx, namespace, project string, ref experience.Reference) (bool, error) {
	if _, err := uuid.Parse(ref.VersionID); err != nil || !experienceErasureHash(ref.BodyHash) || ref.DeliveryRevision < 1 {
		return false, ErrExperienceErasureDeferred
	}
	var authentic, erased, due bool
	err := tx.QueryRow(ctx, `SELECT v.body_hash=$4 AND s.version_id IS NOT NULL AND EXISTS(
 SELECT 1 FROM pangu.experience_deliveries d WHERE d.version_id=v.id AND d.revision=$5 AND d.mode IN ('serving','canary')),
 coalesce(s.tombstoned AND s.validity IN ('revoked','expired'),false) AND NOT EXISTS(SELECT 1 FROM pangu.experience_bodies b WHERE b.version_id=v.id),
 v.expires_at<=clock_timestamp() OR `+experienceErasureConsentSQL+`
 FROM pangu.experience_versions v LEFT JOIN pangu.experience_states s ON s.version_id=v.id
 WHERE v.namespace=$1 AND v.project_id=$2 AND v.id=$3`, namespace, project, ref.VersionID, ref.BodyHash, ref.DeliveryRevision).Scan(&authentic, &erased, &due)
	if err == pgx.ErrNoRows {
		return false, ErrExperienceErasureDeferred
	}
	if err != nil {
		return false, err
	}
	if !authentic {
		return false, ErrExperienceErasureDeferred
	}
	return erased && due, nil
}

func experienceErasureHash(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == sha256.Size && strings.ToLower(value) == value
}
