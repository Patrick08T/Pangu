package scheduler

import (
	"context"
	"sort"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

type experienceErasureBinding struct{ namespace, project, request, run string }

func experienceErasureProofTx(ctx context.Context, tx pgx.Tx, binding experienceErasureBinding) ([]experience.Reference, error) {
	var bound bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.submissions s JOIN pangu.projects p ON p.id=s.project_id AND p.namespace=s.namespace WHERE s.namespace=$1 AND s.project_id=$2 AND s.id=$3 AND coalesce(s.run_id::text,'')=$4 AND coalesce(s.snapshot->>'purpose','') IN ('','business'))`, binding.namespace, binding.project, binding.request, binding.run).Scan(&bound)
	if err != nil {
		return nil, err
	}
	if !bound {
		return nil, ErrExperienceErasureDeferred
	}
	g := &experienceErasureGraph{tx: tx, binding: binding, artifacts: map[string]experience.Artifact{}, visiting: map[string]bool{}, roots: map[string]experience.Reference{}, checked: map[string]bool{}}
	if err = g.usages(ctx); err != nil {
		return nil, err
	}
	if err = g.runArtifacts(ctx); err != nil {
		return nil, err
	}
	if err = g.fork(ctx); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(g.roots))
	for key := range g.roots {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	roots := make([]experience.Reference, 0, len(keys))
	for _, key := range keys {
		roots = append(roots, g.roots[key])
	}
	return roots, nil
}
