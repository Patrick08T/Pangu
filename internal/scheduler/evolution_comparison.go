package scheduler

import (
	"context"
	"errors"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

func evolutionReadObservationTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, id string) (evolution.BusinessEvidence, error) {
	var out evolution.BusinessEvidence
	candidate, err := evolutionBusinessRunTx(ctx, tx, s, id)
	if err != nil {
		return out, err
	}
	var stable, metric string
	err = tx.QueryRow(ctx, `SELECT b.stable_version::text,p.policy#>>'{evaluation,primary_metric}' FROM pangu.evolution_bindings b
 JOIN LATERAL(SELECT * FROM pangu.experience_policies WHERE namespace=b.namespace AND project_id=b.project_id ORDER BY revision DESC LIMIT 1) p ON true
 WHERE b.namespace=$1 AND b.project_id=$2 AND b.entry_key=$3 AND b.adopted_version=$4 AND b.adopted_version<>b.stable_version AND p.actor_id=$5 AND p.policy->>'enabled'='true' AND p.policy->>'revoked'<>'true' FOR SHARE OF b`, s.Namespace, s.ProjectID, candidate.EntryKey, candidate.Snapshot.Workflow.VersionID, s.ActorID).Scan(&stable, &metric)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, evolution.ErrChecker
	}
	if err != nil {
		return out, err
	}
	baseline, err := evolutionBaselineTx(ctx, tx, s, candidate, stable)
	if err != nil {
		return out, err
	}
	if !evolutionComparable(candidate, baseline) {
		return out, evolution.ErrChecker
	}
	left, currency, err := evolutionBusinessScore(ctx, tx, candidate, metric)
	if err != nil {
		return out, err
	}
	right, other, err := evolutionBusinessScore(ctx, tx, baseline, metric)
	if err != nil {
		return out, err
	}
	if currency != other {
		return out, evolution.ErrChecker
	}
	out = evolution.BusinessEvidence{RunID: id, BaselineRunID: baseline.RunID, EntryKey: candidate.EntryKey, VersionID: candidate.Snapshot.Workflow.VersionID, BaselineVersionID: stable, Purpose: "business", Terminal: true, StopConfirmed: true, CandidateScore: left, BaselineScore: right}
	out.SourceHash = evolutionHash(struct {
		Result              evolution.BusinessEvidence
		Candidate, Baseline evolutionBusinessRun
	}{out, candidate, baseline})
	return out, nil
}

func evolutionBaselineTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, current evolutionBusinessRun, version string) (evolutionBusinessRun, error) {
	var id string
	// Choose by pre-outcome identity and recency, never by whether feedback passed.
	err := tx.QueryRow(ctx, `SELECT old.run_id::text FROM pangu.submissions old
 WHERE old.namespace=$1 AND old.project_id=$2 AND old.snapshot#>>'{workflow,version_id}'=$3 AND old.input_hash=$4 AND old.created_by::text=$5 AND old.created_at<$6 AND old.run_id IS NOT NULL AND old.task_id<>$7
 AND coalesce((SELECT 'cron:'||sf.schedule_id::text FROM pangu.schedule_fires sf WHERE sf.request_id=old.id LIMIT 1),(SELECT 'webhook:'||g.hook_id::text FROM pangu.github_events g WHERE g.request_id=old.id LIMIT 1),old.snapshot->>'entry_key','')=$8
 ORDER BY old.created_at DESC,old.id DESC LIMIT 1`, s.Namespace, s.ProjectID, version, current.InputHash, current.OwnerID, current.CreatedAt, current.TaskID, current.EntryKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return evolutionBusinessRun{}, evolution.ErrChecker
	}
	if err != nil {
		return evolutionBusinessRun{}, err
	}
	return evolutionBusinessRunTx(ctx, tx, s, id)
}
