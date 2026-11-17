package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evolution"
)

func (b Backend) evolutionObserveRuns(ctx context.Context, s evolution.Scope, id string, generation int) error {
	cursor := ""
	service := b.EvolutionService()
	for {
		runs, err := evolutionObservationPage(ctx, s, id, generation, cursor)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			intent, err := service.Observe(ctx, s, id, generation, run)
			// Missing comparable evidence is not a passing sample and cannot postpone the observation deadline.
			if err != nil && !errors.Is(err, evolution.ErrChecker) && !errors.Is(err, evolution.ErrConflict) {
				return err
			}
			if intent != nil && intent.State != "observing" {
				return nil
			}
			cursor = run
		}
		if len(runs) < 100 {
			return nil
		}
	}
}

func evolutionObservationPage(ctx context.Context, s evolution.Scope, id string, generation int, cursor string) ([]string, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, `SELECT s.run_id::text FROM pangu.submissions s JOIN pangu.run_history h ON h.run_id=s.run_id AND h.namespace=s.namespace
 JOIN pangu.evolution_intents i ON i.id=$3 AND i.namespace=s.namespace AND i.project_id=s.project_id AND i.generation=$4 AND i.state='observing'
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.purged_at IS NULL AND s.created_at>=i.observing_at AND s.snapshot#>>'{workflow,version_id}'=coalesce(i.publication->>'flow_id',i.publication->>'agent_id')
 AND coalesce(s.snapshot->>'purpose','business')='business' AND ($5='' OR s.run_id>NULLIF($5,'')::uuid)
 AND NOT EXISTS(SELECT 1 FROM pangu.evolution_observations o WHERE o.intent_id=i.id AND o.generation=i.generation AND o.run_id=s.run_id)
 ORDER BY s.run_id LIMIT 100`, s.Namespace, s.ProjectID, id, generation, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []string{}
	for rows.Next() {
		var run string
		if err = rows.Scan(&run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (b Backend) evolutionIntentRemote(ctx context.Context, s evolution.Scope, id string, generation int) error {
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	cursor := ""
	var failures []error
	for {
		rows, err := conn.Query(ctx, `SELECT r.effect_key FROM pangu.evolution_remote_intents r JOIN pangu.evolution_switches sw ON r.effect_key=sw.intent_id::text||':'||sw.generation::text||':'||sw.entry_key||':'||sw.direction WHERE r.namespace=$1 AND r.project_id=$2 AND sw.intent_id=$3 AND sw.generation=$4 AND r.status IN ('pending','running','unknown') AND r.effect_key>$5 ORDER BY r.effect_key LIMIT 100`, s.Namespace, s.ProjectID, id, generation, cursor)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		keys := []string{}
		for rows.Next() {
			var key string
			if err = rows.Scan(&key); err != nil {
				rows.Close()
				return err
			}
			keys = append(keys, key)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, key := range keys {
			if err = b.evolutionReconcileRemote(ctx, key); err != nil {
				failures = append(failures, err)
			}
			cursor = key
		}
		if len(keys) < 100 {
			return errors.Join(failures...)
		}
	}
}
