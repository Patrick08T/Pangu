package evaluation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrHoldoutBudget = errors.New("保留集候选额度已耗尽")

func ReserveExperiment(ctx context.Context, namespace string, spec ExperimentSpec) (*FrozenExperiment, error) {
	return experimentTransaction(ctx, func(tx pgx.Tx) (*FrozenExperiment, error) { return ReserveExperimentTx(ctx, tx, namespace, spec) })
}

// ReserveExperimentTx must commit before any hidden input is disclosed or execution starts.
// A caller must roll back the transaction on error.
func ReserveExperimentTx(ctx context.Context, tx pgx.Tx, namespace string, spec ExperimentSpec) (*FrozenExperiment, error) {
	if namespace == "" || !validExperimentIDs(spec.ID, spec.SuiteID, spec.LogicalProjectID, spec.BaselineVersion) {
		return nil, ErrInvalid
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('evaluation-experiment:'||$1))`, spec.ID); err != nil {
		return nil, err
	}
	s, err := readFrozenSuite(ctx, tx, namespace, spec.SuiteID)
	if err != nil {
		return nil, err
	}
	e, err := FreezeExperiment(s, spec)
	if err != nil {
		return nil, err
	}
	if err = validateSuiteSources(ctx, tx, namespace, s); err != nil {
		return nil, err
	}
	exists, err := existingReservation(ctx, tx, namespace, e)
	if err != nil {
		return nil, err
	}
	if exists {
		return e, nil
	}
	result, err := tx.Exec(ctx, `UPDATE pangu.evaluation_holdout_pools SET used=used+1
   WHERE id=(SELECT pool_id FROM pangu.evaluation_suites WHERE id=$1 AND namespace=$2) AND used<max_candidates`, s.ID, namespace)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() != 1 {
		return nil, ErrHoldoutBudget
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pangu.evaluation_experiments(id,namespace,project_id,suite_id,generation,spec,spec_hash)
   VALUES($1,$2,$3,$4,$5,$6,$7)`, e.ID, namespace, e.LogicalProjectID, e.SuiteID, e.Generation, data, e.Hash)
	return e, err
}

func existingReservation(ctx context.Context, tx pgx.Tx, namespace string, e *FrozenExperiment) (bool, error) {
	var hash, status string
	var generation int
	err := tx.QueryRow(ctx, `SELECT spec_hash,generation,status FROM pangu.evaluation_experiments WHERE namespace=$1 AND id=$2 FOR UPDATE`, namespace, e.ID).Scan(&hash, &generation, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if hash != e.Hash || generation != e.Generation || status != "active" {
		return false, ErrConflict
	}
	return true, nil
}

func readActiveExperiment(ctx context.Context, tx pgx.Tx, namespace, id string) (*FrozenSuite, *FrozenExperiment, error) {
	var data []byte
	var hash, status, project, suite string
	var generation int
	err := tx.QueryRow(ctx, `SELECT spec,spec_hash,generation,status,project_id::text,suite_id::text FROM pangu.evaluation_experiments WHERE namespace=$1 AND id=$2 FOR UPDATE`, namespace, id).Scan(&data, &hash, &generation, &status, &project, &suite)
	if err != nil {
		return nil, nil, experimentDBError(err)
	}
	if status != "active" {
		return nil, nil, ErrConflict
	}
	var spec ExperimentSpec
	if err = json.Unmarshal(data, &spec); err != nil {
		return nil, nil, err
	}
	if spec.ID != id || spec.LogicalProjectID != project || spec.SuiteID != suite {
		return nil, nil, ErrConflict
	}
	s, err := readFrozenSuite(ctx, tx, namespace, spec.SuiteID)
	if err != nil {
		return nil, nil, err
	}
	e, err := FreezeExperiment(s, spec)
	if err != nil {
		return nil, nil, err
	}
	if e.Hash != hash || e.Generation != generation {
		return nil, nil, ErrConflict
	}
	if err = validateSuiteSources(ctx, tx, namespace, s); err != nil {
		return nil, nil, err
	}
	return s, e, nil
}

func ReadExperiment(ctx context.Context, namespace, project, id string) (*FrozenExperiment, error) {
	return experimentTransaction(ctx, func(tx pgx.Tx) (*FrozenExperiment, error) {
		return ReadExperimentTx(ctx, tx, namespace, project, id)
	})
}

// ReadExperimentTx locks the experiment and authenticates every live suite source
// in the caller's transaction. Publication must also fence live policy/permissions.
func ReadExperimentTx(ctx context.Context, tx pgx.Tx, namespace, project, id string) (*FrozenExperiment, error) {
	_, e, err := readActiveExperiment(ctx, tx, namespace, id)
	if err != nil {
		return nil, err
	}
	if e.LogicalProjectID != project {
		return nil, ErrNotFound
	}
	return e, nil
}

func LoadExperimentCase(ctx context.Context, namespace, experimentID, caseID string) (*Case, error) {
	return experimentTransaction(ctx, func(tx pgx.Tx) (*Case, error) { return LoadExperimentCaseTx(ctx, tx, namespace, experimentID, caseID) })
}

// LoadExperimentCaseTx is trusted evaluator-only: expected/criteria must never
// enter the candidate payload. The reservation must already be durably committed.
func LoadExperimentCaseTx(ctx context.Context, tx pgx.Tx, namespace, experimentID, caseID string) (*Case, error) {
	s, e, err := readActiveExperiment(ctx, tx, namespace, experimentID)
	if err != nil {
		return nil, err
	}
	// A non-locking, separate MVCC read proves the exposure spend is committed,
	// including when this transaction has nested savepoints. Source checks and
	// all authoritative locks remain in tx; no second locking transaction opens.
	committed, err := committedReservation(ctx, namespace, e)
	if err != nil {
		return nil, err
	}
	if !committed {
		return nil, ErrConflict
	}
	for _, c := range s.Cases {
		if c.ID == caseID && c.Partition != "development" {
			return loadSuiteSource(ctx, tx, namespace, s.ProjectID, c)
		}
	}
	return nil, ErrNotFound
}

// InvalidateExperimentTx retains the row fence until the caller commits or rolls back.
func InvalidateExperimentTx(ctx context.Context, tx pgx.Tx, namespace, id string, generation int) error {
	result, err := tx.Exec(ctx, `UPDATE pangu.evaluation_experiments SET status='invalidated',generation=generation+1 WHERE namespace=$1 AND id=$2 AND generation=$3 AND status='active'`, namespace, id, generation)
	if err == nil && result.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func InvalidateExperiment(ctx context.Context, namespace, id string, generation int) error {
	_, err := experimentTransaction(ctx, func(tx pgx.Tx) (bool, error) {
		err := InvalidateExperimentTx(ctx, tx, namespace, id, generation)
		return err == nil, err
	})
	return err
}
