package experience

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Only a trusted machine-evidence writer sets this private delivery JSON flag.
// It is sticky across delivery revisions; public DTOs cannot set or clear it.
func frozenSafetyBlockedTx(ctx context.Context, tx pgx.Tx, versionID string) (bool, error) {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.experience_deliveries
 WHERE version_id=$1 AND policy->'block_frozen'='true'::jsonb)`, versionID).Scan(&blocked)
	return blocked, err
}

func checkFrozenSafetyTx(ctx context.Context, tx pgx.Tx, versionID string) error {
	blocked, err := frozenSafetyBlockedTx(ctx, tx, versionID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrInvalidated
	}
	return nil
}
