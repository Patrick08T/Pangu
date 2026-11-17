package learning

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

var errObservationSource = errors.New("memory observation source unavailable")

type memoryWindow struct{ Origin, Start, End, Deadline, Now time.Time }

func readMemoryWindow(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy) (memoryWindow, error) {
	var out memoryWindow
	err := tx.QueryRow(ctx, `SELECT created_at,clock_timestamp() FROM pangu.experience_deliveries WHERE version_id=$1 AND revision=$2`, version.ID, version.DeliveryRevision).Scan(&out.Origin, &out.Now)
	if err != nil {
		return out, err
	}
	out.Start = out.Origin
	if version.Delivery == "serving" {
		var last *time.Time
		err = tx.QueryRow(ctx, `SELECT max((detail->>'window_end')::timestamptz) FROM pangu.learning_events WHERE namespace=$1 AND project_id=$2 AND subject_id=$3 AND action='memory_window_healthy'
   AND (detail->>'delivery_revision')::bigint=$4 AND (detail->>'policy_revision')::bigint=$5`, scope.Namespace, scope.ProjectID, version.ID, version.DeliveryRevision, policy.Revision).Scan(&last)
		if err != nil {
			return out, err
		}
		if last != nil && last.After(out.Start) {
			out.Start = *last
		}
	}
	out.End = out.Start.Add(time.Duration(policy.Observation.WindowSeconds) * time.Second)
	out.Deadline = out.Start.Add(time.Duration(policy.Observation.DeadlineSeconds) * time.Second)
	return out, nil
}

func memoryWindowAssessment(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, evidence string) (bool, error) {
	policy, err := policyTx(ctx, tx, scope)
	if err != nil {
		return false, err
	}
	window, err := readMemoryWindow(ctx, tx, scope, version, policy)
	if err != nil {
		return false, err
	}
	if window.Now.Before(window.End) {
		return false, nil
	}
	runs, err := loadObservationRuns(ctx, tx, scope, version, window.Start, window.End)
	if err != nil {
		return false, err
	}
	extended := policy
	extended.Observation.WindowSeconds = int64(window.End.Sub(window.Origin) / time.Second)
	extended.Observation.DeadlineSeconds = int64(window.Deadline.Sub(window.Origin) / time.Second)
	cohort, err := cohortValues(ctx, tx, scope, version, extended, runs)
	if err != nil {
		if !memoryGuardFailure(err) {
			return false, err
		}
		if errors.Is(err, errObservationSource) {
			return pauseMemoryTx(ctx, tx, scope, version.ID, version.DeliveryRevision, "observation_source_invalid")
		}
		return incompleteMemoryWindow(ctx, tx, scope, version, window)
	}
	experiment, err := evaluation.ReadExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, evidence)
	if err != nil {
		return false, err
	}
	suite, err := evaluation.ReadExperimentSuiteTx(ctx, tx, scope.Namespace, experiment.SuiteID)
	if err != nil {
		return false, err
	}
	baseline, err := observationBaseline(ctx, tx, suite, experiment)
	if err != nil {
		return false, err
	}
	if observationDegraded(policy, cohort, baseline) {
		return pauseMemoryTx(ctx, tx, scope, version.ID, version.DeliveryRevision, "confirmed_degradation")
	}
	if !observationNonInferior(policy, cohort, baseline) {
		return incompleteMemoryWindow(ctx, tx, scope, version, window)
	}
	scope.Action = "memory_window_healthy"
	err = auditTx(ctx, tx, scope, version.ID, map[string]any{"delivery_revision": version.DeliveryRevision, "policy_revision": policy.Revision, "window_end": window.End, "cohort_size": len(cohort)})
	return false, err
}

func incompleteMemoryWindow(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, window memoryWindow) (bool, error) {
	if window.Now.Before(window.Deadline) {
		return false, nil
	}
	return pauseMemoryTx(ctx, tx, scope, version.ID, version.DeliveryRevision, "observation_deadline_insufficient")
}
