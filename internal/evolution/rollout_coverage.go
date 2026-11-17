package evolution

import (
	"context"
	"time"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

// ObservationCoverageReader proves that every possibly accepted candidate run
// has a current validated observation. Implementations must retain the shared
// acceptance locks through this transaction; missing provenance is not absence.
type ObservationCoverageReader interface {
	ObservationCoverageTx(context.Context, pgx.Tx, Scope, string, int) (bool, error)
}

func (s Service) rolloutCoverageTx(ctx context.Context, tx pgx.Tx, scope Scope, intent *intentRecord, policy experience.Policy, now time.Time) (bool, error) {
	complete := false
	if reader, ok := s.Evidence.(ObservationCoverageReader); ok {
		var err error
		complete, err = reader.ObservationCoverageTx(ctx, tx, scope, intent.ID, intent.Generation)
		if err != nil {
			return false, err
		}
	}
	if complete {
		return true, nil
	}
	if now.Sub(intent.StartedAt) < time.Duration(policy.Observation.DeadlineSeconds)*time.Second {
		return false, nil
	}
	return false, rolloutBeginRollback(ctx, tx, scope, intent, "最长观察期限内业务请求覆盖不完整", "insufficient")
}
