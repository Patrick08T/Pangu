package scheduler

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func (g *experienceErasureGraph) usages(ctx context.Context) error {
	rows, err := g.tx.Query(ctx, `SELECT jsonb_build_object('references',usage->'references','injection_hash',usage->'injection_hash',
 'watermark_hash',usage->'watermark_hash','input',jsonb_build_object('project_id',usage#>'{input,project_id}',
 'run_id',usage#>'{input,run_id}','generation',usage#>'{input,generation}','watermark_generation',usage#>'{input,watermark_generation}',
 'round',usage#>'{input,round}','approval_revision',usage#>'{input,approval_revision}','bound_input_hash',usage#>'{input,bound_input_hash}'))
 FROM pangu.experience_usage WHERE namespace=$1 AND project_id=$2 AND usage#>>'{input,run_id}'=$3 ORDER BY identity_hash LIMIT 1025`, g.binding.namespace, g.binding.project, g.binding.request)
	if err != nil {
		return err
	}
	usages, err := pgx.CollectRows(rows, pgx.RowTo[experience.Usage])
	if err != nil {
		return err
	}
	if len(usages) > 1024 {
		return ErrExperienceErasureDeferred
	}
	for _, u := range usages {
		if len(u.References) == 0 {
			continue
		}
		if err = g.usage(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

func (g *experienceErasureGraph) usage(ctx context.Context, u experience.Usage) error {
	in := u.Input
	if in.ProjectID != g.binding.project || in.RunID != g.binding.request || in.WatermarkGeneration != 1 || in.Generation < 1 || in.Round < 1 || in.ApprovalRevision < 1 || !experienceErasureHash(in.BoundInputHash) || u.InjectionHash != experienceErasureDigest(u.References) {
		return ErrExperienceErasureDeferred
	}
	var mark experience.Watermark
	err := g.tx.QueryRow(ctx, `SELECT snapshot FROM pangu.experience_watermarks WHERE namespace=$1 AND project_id=$2 AND run_id=$3 AND generation=1`, g.binding.namespace, g.binding.project, g.binding.request).Scan(&mark)
	if err == pgx.ErrNoRows {
		return ErrExperienceErasureDeferred
	}
	if err != nil {
		return err
	}
	hash := mark.Hash
	mark.Hash = ""
	if mark.RunID != g.binding.request || mark.Generation != 1 || u.WatermarkHash != hash || experienceErasureDigest(mark) != hash {
		return ErrExperienceErasureDeferred
	}
	for _, ref := range u.References {
		if !slices.ContainsFunc(mark.Versions, func(v experience.FrozenVersion) bool { return v.Reference == ref }) {
			return ErrExperienceErasureDeferred
		}
	}
	return g.references(ctx, u.References)
}

func (g *experienceErasureGraph) runArtifacts(ctx context.Context) error {
	rows, err := g.tx.Query(ctx, `SELECT id,artifact->>'hash' FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND artifact->>'run_id'=$3 ORDER BY id LIMIT 1025`, g.binding.namespace, g.binding.project, g.binding.run)
	if err != nil {
		return err
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[experience.ArtifactReference])
	if err != nil {
		return err
	}
	if len(refs) > 1024 {
		return ErrExperienceErasureDeferred
	}
	for _, ref := range refs {
		if _, err = g.visit(ctx, ref, 0); err != nil {
			return err
		}
	}
	return nil
}
