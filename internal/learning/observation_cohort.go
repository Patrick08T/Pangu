package learning

import (
	"context"
	"time"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

type observedRun struct {
	RunID    string
	Snapshot []byte
}

func observationCohort(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy) ([]observedRun, error) {
	var start, now time.Time
	err := tx.QueryRow(ctx, `SELECT created_at,clock_timestamp() FROM pangu.experience_deliveries WHERE version_id=$1 AND revision=$2 AND mode='canary'`, version.ID, version.DeliveryRevision).Scan(&start, &now)
	if err != nil {
		return nil, err
	}
	end := start.Add(time.Duration(policy.Observation.WindowSeconds) * time.Second)
	if now.Before(end) || now.After(start.Add(time.Duration(policy.Observation.DeadlineSeconds)*time.Second)) {
		return nil, experience.ErrForbidden
	}
	return loadObservationRuns(ctx, tx, scope, version, start, end)
}

func loadObservationRuns(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, start, end time.Time) ([]observedRun, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT coalesce(s.run_id::text,''),coalesce(s.snapshot,'{}'::jsonb)
 FROM pangu.experience_usage u LEFT JOIN pangu.submissions s ON s.id::text=u.usage->'input'->>'run_id' AND s.namespace=u.namespace AND s.project_id=u.project_id
 WHERE u.namespace=$1 AND u.project_id=$2 AND EXISTS(SELECT 1 FROM jsonb_array_elements(u.usage->'references') r WHERE r->>'version_id'=$3 AND (r->>'delivery_revision')::bigint=$4)
 AND (u.usage->>'created_at')::timestamptz >= $5 AND (u.usage->>'created_at')::timestamptz < $6`, scope.Namespace, scope.ProjectID, version.ID, version.DeliveryRevision, start, end)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (observedRun, error) {
		var out observedRun
		err := row.Scan(&out.RunID, &out.Snapshot)
		return out, err
	})
}

func cohortValues(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy, runs []observedRun) (map[string]bool, error) {
	cohort := map[string]bool{}
	tasks := map[string]string{}
	for _, run := range runs {
		if !validID(run.RunID) {
			return nil, experience.ErrInvalidated
		}
		observation, err := deriveObservation(ctx, tx, scope, version, policy, run.RunID)
		if err != nil {
			return nil, err
		}
		input := frozenInputHash(run.Snapshot)
		if !validHash(input) {
			return nil, experience.ErrInvalidated
		}
		if prior, ok := tasks[observation.TaskID]; ok && prior != input {
			return nil, experience.ErrInvalidated
		}
		tasks[observation.TaskID] = input
		if prior, ok := cohort[input]; ok {
			cohort[input] = prior && observation.Accepted
		} else {
			cohort[input] = observation.Accepted
		}
	}
	return cohort, nil
}

func verifyMemoryObservations(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, policy experience.Policy, suite *evaluation.FrozenSuite, experiment *evaluation.FrozenExperiment) error {
	runs, err := observationCohort(ctx, tx, scope, version, policy)
	if err != nil {
		return err
	}
	cohort, err := cohortValues(ctx, tx, scope, version, policy, runs)
	if err != nil {
		return err
	}
	baseline, err := observationBaseline(ctx, tx, suite, experiment)
	if err != nil {
		return err
	}
	if !observationNonInferior(policy, cohort, baseline) {
		return experience.ErrForbidden
	}
	return auditTx(ctx, tx, scope, version.ID, map[string]any{"delivery_revision": version.DeliveryRevision, "cohort_size": len(cohort), "policy_revision": policy.Revision})
}
