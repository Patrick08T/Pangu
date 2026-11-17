package evaluation

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func CreateExperimentSuite(ctx context.Context, namespace string, spec SuiteSpec) (*FrozenSuite, error) {
	s, err := FreezeSuite(spec)
	if err != nil {
		return nil, err
	}
	if namespace == "" || !validExperimentIDs(s.ID, s.ProjectID) {
		return nil, ErrInvalid
	}
	return experimentTransaction(ctx, func(tx pgx.Tx) (*FrozenSuite, error) {
		// Classification and learning disclosure share this fence. Acquire it
		// before case/source/pool row locks, matching identity -> experience order.
		if err := identity.Lock(ctx, tx, namespace); err != nil {
			return nil, err
		}
		if err := validateSuiteSources(ctx, tx, namespace, s); err != nil {
			return nil, err
		}
		pool, err := ensureHoldoutPool(ctx, tx, namespace, s)
		if err != nil {
			return nil, err
		}
		for _, c := range s.Cases {
			if err = allocateSuiteCase(ctx, tx, namespace, s.ProjectID, pool, c); err != nil {
				return nil, err
			}
		}
		data, err := json.Marshal(s.SuiteSpec)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO pangu.evaluation_suites(id,namespace,project_id,pool_id,spec,spec_hash)
   VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO NOTHING`, s.ID, namespace, s.ProjectID, pool, data, s.Hash)
		if err != nil {
			return nil, err
		}
		saved, err := readFrozenSuite(ctx, tx, namespace, s.ID)
		if err != nil {
			return nil, err
		}
		if saved.Hash != s.Hash {
			return nil, ErrConflict
		}
		for _, c := range s.Cases {
			_, err = tx.Exec(ctx, `INSERT INTO pangu.evaluation_suite_cases(suite_id,case_id,partition,input_hash,family_id,task_id,source_run_id)
    VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, s.ID, c.ID, c.Partition, c.InputHash, c.FamilyID, c.TaskID, c.SourceRunID)
			if err != nil {
				return nil, err
			}
		}
		return s, nil
	})
}

func ensureHoldoutPool(ctx context.Context, tx pgx.Tx, namespace string, s *FrozenSuite) (string, error) {
	rules := contentHash(s.Rules)
	_, err := tx.Exec(ctx, `INSERT INTO pangu.evaluation_holdout_pools(id,namespace,project_id,holdout_hash,rules_hash,max_candidates)
  VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(namespace,project_id,holdout_hash) DO NOTHING`, uuid.NewString(), namespace, s.ProjectID, s.HoldoutHash, rules, s.Rules.MaxCandidates)
	if err != nil {
		return "", err
	}
	var id, saved string
	err = tx.QueryRow(ctx, `SELECT id::text,rules_hash FROM pangu.evaluation_holdout_pools WHERE namespace=$1 AND project_id=$2 AND holdout_hash=$3 FOR UPDATE`, namespace, s.ProjectID, s.HoldoutHash).Scan(&id, &saved)
	if err != nil {
		return "", err
	}
	if saved != rules {
		return "", ErrConflict
	}
	return id, nil
}

func readFrozenSuite(ctx context.Context, tx pgx.Tx, namespace, id string) (*FrozenSuite, error) {
	var data []byte
	var hash string
	err := tx.QueryRow(ctx, `SELECT spec,spec_hash FROM pangu.evaluation_suites WHERE namespace=$1 AND id=$2`, namespace, id).Scan(&data, &hash)
	if err != nil {
		return nil, experimentDBError(err)
	}
	var spec SuiteSpec
	if err = json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	s, err := FreezeSuite(spec)
	if err != nil {
		return nil, err
	}
	if s.Hash != hash {
		return nil, ErrConflict
	}
	return s, nil
}

func ReadExperimentSuite(ctx context.Context, namespace, id string) (*FrozenSuite, error) {
	return experimentTransaction(ctx, func(tx pgx.Tx) (*FrozenSuite, error) { return ReadExperimentSuiteTx(ctx, tx, namespace, id) })
}

func ReadExperimentSuiteTx(ctx context.Context, tx pgx.Tx, namespace, id string) (*FrozenSuite, error) {
	s, err := readFrozenSuite(ctx, tx, namespace, id)
	if err != nil {
		return nil, err
	}
	if err = validateSuiteSources(ctx, tx, namespace, s); err != nil {
		return nil, err
	}
	return s, nil
}
