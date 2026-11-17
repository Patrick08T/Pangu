package evolution

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rolloutSample struct {
	Observation
	Evidence BusinessEvidence `json:"evidence"`
}

func (s Service) rolloutObserve(ctx context.Context, scope Scope, id string, generation int, runID string) (*Intent, error) {
	if s.Evidence == nil {
		return nil, ErrChecker
	}
	if _, err := uuid.Parse(runID); err != nil {
		return nil, ErrInvalid
	}
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, policy, err := authorizedIntentTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = policy.ActorID
		if intent.State != "observing" {
			return ErrConflict
		}
		evidence, err := s.Evidence.ReadObservationTx(ctx, tx, scope, runID)
		if err != nil {
			return err
		}
		sample, err := rolloutValidateEvidence(intent, policy, evidence, runID)
		if err != nil {
			return err
		}
		if err = rolloutInsertSample(ctx, tx, intent, sample); err != nil {
			return err
		}
		if sample.HardFailure {
			if err = rolloutBeginRollback(ctx, tx, scope, intent, "实际业务触发关键保护失败", "rolled_back"); err != nil {
				return err
			}
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func rolloutValidateEvidence(intent *intentRecord, policy experience.Policy, e BusinessEvidence, runID string) (rolloutSample, error) {
	sample := rolloutSample{Evidence: e}
	decoded, hashErr := hex.DecodeString(e.SourceHash)
	if hashErr != nil || len(decoded) != 32 {
		return sample, ErrChecker
	}
	original, err := rolloutEntry(intent, e.EntryKey)
	if err != nil {
		return sample, ErrChecker
	}
	if e.RunID != runID || e.BaselineRunID == e.RunID || e.Purpose != "business" || !e.Terminal || !e.StopConfirmed || e.VersionID != rolloutVersion(intent) || e.BaselineVersionID != original.VersionID || len(e.SourceHash) != 64 {
		return sample, ErrChecker
	}
	if _, err = uuid.Parse(e.BaselineRunID); err != nil {
		return sample, ErrChecker
	}
	if math.IsNaN(e.CandidateScore) || math.IsInf(e.CandidateScore, 0) || math.IsNaN(e.BaselineScore) || math.IsInf(e.BaselineScore, 0) || e.CandidateScore < 0 || e.BaselineScore < 0 {
		return sample, ErrChecker
	}
	sample.Delta = e.CandidateScore - e.BaselineScore
	sample.HardFailure = e.HardFailure
	switch policy.Evaluation.PrimaryMetric {
	case "acceptance":
		if e.CandidateScore > 1 || e.BaselineScore > 1 {
			return sample, ErrChecker
		}
	case "cost", "duration", "rework":
		sample.Delta = -sample.Delta
	default:
		return sample, ErrChecker
	}
	return sample, nil
}

func rolloutInsertSample(ctx context.Context, tx pgx.Tx, intent *intentRecord, sample rolloutSample) error {
	e := sample.Evidence
	var duplicate bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_observations WHERE intent_id=$1 AND generation=$2 AND run_id<>$3 AND (sample#>>'{evidence,BaselineRunID}'=$4 OR source_hash=$5))`, intent.ID, intent.Generation, e.RunID, e.BaselineRunID, e.SourceHash).Scan(&duplicate)
	if err != nil {
		return err
	}
	if duplicate {
		return ErrConflict
	}
	tag, err := tx.Exec(ctx, `INSERT INTO pangu.evolution_observations(intent_id,generation,entry_key,run_id,source_hash,sample) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, intent.ID, intent.Generation, e.EntryKey, e.RunID, e.SourceHash, jsonBytes(sample))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var existing []byte
	if err = tx.QueryRow(ctx, `SELECT sample FROM pangu.evolution_observations WHERE intent_id=$1 AND generation=$2 AND entry_key=$3 AND run_id=$4`, intent.ID, intent.Generation, e.EntryKey, e.RunID).Scan(&existing); err != nil {
		return err
	}
	var old rolloutSample
	if err = json.Unmarshal(existing, &old); err != nil {
		return err
	}
	if hashJSON(old) != hashJSON(sample) {
		return ErrConflict
	}
	return nil
}

func (s Service) rolloutAdvance(ctx context.Context, scope Scope, id string, generation int) (*Intent, error) {
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		prior, _, err := rolloutRecoveryTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		if prior.State == "stable" || prior.State == "rolled_back" || prior.State == "insufficient" {
			out = &prior.Intent
			return nil
		}
		intent, policy, err := authorizedIntentTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = policy.ActorID
		if intent.State == "rolling_back" || intent.State == "reconciling" {
			out = &intent.Intent
			return nil
		}
		if intent.State != "observing" {
			return ErrConflict
		}
		if err = s.publishedForwardTx(ctx, tx, scope, intent, policy); err != nil {
			if err = publishedRejectTx(ctx, tx, scope, intent, err); err != nil {
				return err
			}
			out = &intent.Intent
			return nil
		}
		if err = s.rolloutAssess(ctx, tx, scope, intent, policy); err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}

func (s Service) rolloutAssess(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy) error {
	cohort, invalid, err := s.rolloutRevalidatedSamples(ctx, tx, scope, intent, policy)
	if err != nil {
		return err
	}
	if invalid {
		return rolloutBeginRollback(ctx, tx, scope, intent, "业务观测来源已失效或发生修订，冻结证据不足", "insufficient")
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	allStable := true
	for _, entry := range intent.Config.Entries {
		outcome, err := observationOutcome(policy.Observation, intent.StartedAt, now, cohort[entry.Target.Key()])
		if err != nil {
			return err
		}
		if outcome == "rollback" {
			return rolloutBeginRollback(ctx, tx, scope, intent, "可比业务结果达到退化门槛", "rolled_back")
		}
		if outcome == "insufficient" {
			return rolloutBeginRollback(ctx, tx, scope, intent, "最长观察期限内证据不足", "insufficient")
		}
		if outcome != "stable" {
			allStable = false
		}
	}
	if !allStable {
		return nil
	}
	complete, err := s.rolloutCoverageTx(ctx, tx, scope, intent, policy, now)
	if err != nil || !complete {
		return err
	}
	for _, entry := range intent.Config.Entries {
		if err := s.rolloutCompleteTx(ctx, tx, scope, intent, entry.Target.Key(), "adopt"); err != nil {
			return err
		}
		row, err := rolloutReadSwitch(ctx, tx, intent.ID, intent.Generation, entry.Target.Key(), "adopt")
		if err != nil {
			return err
		}
		if row.Status == "conflict" {
			return rolloutBeginRollback(ctx, tx, scope, intent, "稳定晋级前入口被人工修改", "rolled_back")
		}
		if row.Status != "confirmed" {
			return nil
		}
	}
	return rolloutFinishTx(ctx, tx, scope, intent, true)
}
