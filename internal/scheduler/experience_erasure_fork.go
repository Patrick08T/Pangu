package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type experienceErasureFork struct {
	source  string
	lineage workflow.OutputLineage
}

func (g *experienceErasureGraph) fork(ctx context.Context) error {
	rows, err := g.tx.Query(ctx, `SELECT coalesce(s.snapshot->>'source_run_id',''),coalesce(value->'lineage','{}'::jsonb)
 FROM pangu.submissions s CROSS JOIN LATERAL jsonb_each(coalesce(s.snapshot#>'{fork,reused}','{}'::jsonb))
 WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 ORDER BY key LIMIT 1025`, g.binding.namespace, g.binding.project, g.binding.request)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (experienceErasureFork, error) {
		var item experienceErasureFork
		err := row.Scan(&item.source, &item.lineage)
		return item, err
	})
	if err != nil {
		return err
	}
	if len(items) > 1024 {
		return ErrExperienceErasureDeferred
	}
	for _, item := range items {
		source, parseErr := uuid.Parse(item.source)
		line := item.lineage
		if parseErr != nil || line.Version != 1 || !experienceErasureHash(line.InputHash) || !experienceErasureHash(line.ExecutionInputHash) {
			return ErrExperienceErasureDeferred
		}
		a, err := g.visit(ctx, experience.ArtifactReference{ID: line.ArtifactID, Hash: line.OutputHash}, 0)
		if err != nil {
			return err
		}
		if a.RunID != source.String() || a.InputHash != line.InputHash || a.ExecutionInputHash != line.ExecutionInputHash {
			return ErrExperienceErasureDeferred
		}
	}
	return nil
}
