package evaluation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
)

func textHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func validateSuiteSources(ctx context.Context, tx pgx.Tx, namespace string, s *FrozenSuite) error {
	for _, c := range s.Cases {
		if _, err := loadSuiteSource(ctx, tx, namespace, s.ProjectID, c); err != nil {
			return err
		}
	}
	return nil
}

func loadSuiteSource(ctx context.Context, tx pgx.Tx, namespace, project string, c SuiteCase) (*Case, error) {
	if !validExperimentIDs(c.ID, c.TaskID, c.SourceRunID) {
		return nil, ErrInvalid
	}
	if err := rejectLearningExposure(ctx, tx, namespace, project, c); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `SELECT c.id::text,c.project_id::text,c.actor_id::text,c.name,c.input,c.expected,c.criteria,
 false,c.created_at,c.expires_at FROM pangu.evaluation_cases c JOIN pangu.projects p ON p.id=c.project_id
 WHERE c.namespace=$1 AND p.namespace=$1 AND c.project_id=$2 AND c.id=$3 AND NOT c.archived AND c.expires_at>now() FOR SHARE OF c`, namespace, project, c.ID)
	stored, err := scanCase(row)
	if err != nil {
		return nil, err
	}
	if textHash(stored.Input) != c.InputHash || textHash(stored.Expected) != c.ExpectedHash || contentHash(stored.Criteria) != c.CriteriaHash {
		return nil, ErrConflict
	}
	var source string
	err = tx.QueryRow(ctx, `SELECT s.id::text FROM pangu.submissions s WHERE s.namespace=$1 AND s.project_id=$2
 AND s.task_id=$3 AND s.run_id=$4 AND s.input_hash=$5 AND s.snapshot->'workflow'->>'input'=$6
 AND NOT EXISTS(SELECT 1 FROM pangu.retention_records r WHERE r.run_id=s.run_id AND r.status='purged')
 AND NOT EXISTS(SELECT 1 FROM pangu.run_debug d WHERE d.run_id=s.run_id AND d.expired) FOR SHARE OF s`,
		namespace, project, c.TaskID, c.SourceRunID, c.InputHash, stored.Input).Scan(&source)
	if err != nil {
		return nil, experimentDBError(err)
	}
	return stored, nil
}
