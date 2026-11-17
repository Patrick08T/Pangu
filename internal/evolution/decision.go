package evolution

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/jackc/pgx/v5"
)

func (s Service) Decide(ctx context.Context, scope Scope, id string, generation int) (evaluation.ExperimentDecision, error) {
	var out evaluation.ExperimentDecision
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, p, err := authorizedIntentTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if !slices.Contains([]string{"evaluating", "passed"}, intent.State) && !(intent.State == "stable" && intent.Config.Request.Kind == "memory_benefit") {
			return ErrConflict
		}
		live, err := s.liveExperimentTx(ctx, tx, scope, intent, p)
		if err != nil {
			return err
		}
		out, err = evaluation.EvaluateStoredExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, intent.ExperimentID, live.Frozen.ExperimentSpec)
		if err != nil {
			return err
		}
		if out.Outcome != "passed" && out.Category != "incomplete_pairs" {
			return s.cancelEvaluationTx(ctx, tx, scope, intent, "独立评测未通过: "+out.Category)
		}
		if out.Outcome == "passed" && live.Frozen.Kind == "memory_benefit" {
			if intent.State == "stable" {
				return nil
			}
			return finishBenefitTx(ctx, tx, scope, intent)
		}
		if out.Outcome == "passed" && intent.State != "passed" {
			return setIntentStateTx(ctx, tx, intent, "passed")
		}
		return nil
	})
	return out, err
}

func (s Service) Promote(ctx context.Context, scope Scope, id string, generation int) (*Intent, error) {
	var out *Intent
	err := transaction(ctx, func(tx pgx.Tx) error {
		intent, p, err := authorizedIntentTx(ctx, tx, scope, id, generation)
		if err != nil {
			return err
		}
		scope.ActorID = p.ActorID
		if intent.Config.Request.Kind == "memory_benefit" {
			return ErrForbidden
		}
		if !slices.Contains([]string{"passed", "switching"}, intent.State) {
			return ErrConflict
		}
		live, err := s.liveExperimentTx(ctx, tx, scope, intent, p)
		if err != nil {
			return err
		}
		decision, err := evaluation.EvaluateStoredExperimentTx(ctx, tx, scope.Namespace, scope.ProjectID, intent.ExperimentID, live.Frozen.ExperimentSpec)
		if err != nil {
			return err
		}
		if decision.Outcome != "passed" {
			return ErrConflict
		}
		if intent.State == "switching" {
			if intent.Publication == nil {
				return ErrConflict
			}
			out = &intent.Intent
			return nil
		}
		guard := s.publicationGuard(scope, intent, live.Frozen.Hash)
		published, err := catalog.PublishAutomaticTx(ctx, tx, scope.Namespace, intent.Config.Publish, guard)
		if err != nil {
			return err
		}
		intent.Publication = &PublishedVersions{AgentID: published.Agent.ID}
		if published.Flow != nil {
			intent.Publication.FlowID = published.Flow.ID
		}
		if _, err = tx.Exec(ctx, `UPDATE pangu.evolution_intents SET publication=$2 WHERE id=$1`, intent.ID, jsonBytes(intent.Publication)); err != nil {
			return err
		}
		if err = setIntentStateTx(ctx, tx, intent, "switching"); err != nil {
			return err
		}
		out = &intent.Intent
		return nil
	})
	return out, err
}
