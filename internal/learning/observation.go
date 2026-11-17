package learning

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
	"time"
)

type MemoryObservation struct {
	ID               string            `json:"id"`
	VersionID        string            `json:"version_id"`
	DeliveryRevision int64             `json:"delivery_revision"`
	PolicyRevision   int64             `json:"policy_revision"`
	RunID            string            `json:"run_id"`
	TaskID           string            `json:"task_id"`
	UsageHash        string            `json:"usage_hash"`
	Source           experience.Source `json:"source"`
	Accepted         bool              `json:"accepted"`
	FinishedAt       time.Time         `json:"finished_at"`
}

// ObserveMemory derives one immutable task observation from original business evidence.
func ObserveMemory(ctx context.Context, scope experience.Scope, versionID, runID string) (MemoryObservation, error) {
	return transaction(ctx, scope, "observation_write", func(tx pgx.Tx, scope experience.Scope) (MemoryObservation, error) {
		version, err := NewStore().ReadTx(ctx, tx, scope, versionID)
		if err != nil {
			return MemoryObservation{}, err
		}
		policy, err := policyTx(ctx, tx, scope)
		if err != nil {
			return MemoryObservation{}, err
		}
		observation, err := deriveObservation(ctx, tx, scope, version, policy, runID)
		if err != nil {
			return observation, err
		}
		return persistObservation(ctx, tx, scope, observation)
	})
}
