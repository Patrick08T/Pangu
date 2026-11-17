package evolution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type rolloutStoredSample struct {
	EntryKey, RunID, SourceHash string
	Sample                      rolloutSample
}

func rolloutStoredSamples(ctx context.Context, tx pgx.Tx, intent *intentRecord) ([]rolloutStoredSample, error) {
	rows, err := tx.Query(ctx, `SELECT entry_key,run_id::text,source_hash,sample FROM pangu.evolution_observations WHERE intent_id=$1 AND generation=$2 ORDER BY created_at,entry_key,run_id`, intent.ID, intent.Generation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []rolloutStoredSample{}
	for rows.Next() {
		var data []byte
		var sample rolloutStoredSample
		if err = rows.Scan(&sample.EntryKey, &sample.RunID, &sample.SourceHash, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &sample.Sample); err != nil {
			return nil, err
		}
		result = append(result, sample)
	}
	return result, rows.Err()
}

func (s Service) rolloutRevalidatedSamples(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy) (map[string][]Observation, bool, error) {
	// 先取完整cohort并关闭rows，再在同一事务内通过可信读取器锁定、复核当前来源。
	stored, err := rolloutStoredSamples(ctx, tx, intent)
	if err != nil {
		return nil, false, err
	}
	if len(stored) > 0 && s.Evidence == nil {
		return nil, false, ErrChecker
	}
	invalid := false
	for _, old := range stored {
		current, err := s.Evidence.ReadObservationTx(ctx, tx, scope, old.RunID)
		if err != nil {
			if !rolloutEvidenceInvalid(err) {
				return nil, false, err
			}
			invalid = true
			continue
		}
		fresh, err := rolloutValidateEvidence(intent, policy, current, old.RunID)
		if err != nil || old.EntryKey != current.EntryKey || old.SourceHash != current.SourceHash || old.Sample != fresh {
			invalid = true
		}
	}
	// 不能因某条失效提前做决定：后续入口的未知故障必须使整个事务失败。
	if invalid {
		return nil, true, nil
	}
	cohort := make(map[string][]Observation)
	for _, old := range stored {
		cohort[old.EntryKey] = append(cohort[old.EntryKey], old.Sample.Observation)
	}
	return cohort, false, nil
}

func rolloutEvidenceInvalid(err error) bool {
	var sql *pgconn.PgError
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &sql) {
		return false
	}
	return errors.Is(err, ErrChecker) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrConflict) || errors.Is(err, experience.ErrInvalidated) || errors.Is(err, experience.ErrNotFound) || errors.Is(err, experience.ErrForbidden) || errors.Is(err, experience.ErrConflict)
}
