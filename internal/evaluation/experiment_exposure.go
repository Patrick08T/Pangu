package evaluation

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Exposure is permanent: withdrawal, retention cleanup, or renamed cases cannot
// make learned data unseen. Unknown history therefore excludes this project's
// hidden sets rather than pretending missing payloads prove independence.
func rejectLearningExposure(ctx context.Context, tx pgx.Tx, namespace, project string, c SuiteCase) error {
	if c.Partition == "development" {
		return nil
	}
	var exposed bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.learning_source_exposures
 WHERE namespace=$1 AND project_id=$2 AND (unresolved OR task_id IS NULL OR input_hash IS NULL
 OR run_id=$3 OR task_id=$4 OR input_hash=$5))`, namespace, project, c.SourceRunID, c.TaskID, c.InputHash).Scan(&exposed)
	if err != nil {
		return err
	}
	if exposed {
		return ErrInvalid
	}
	return nil
}
