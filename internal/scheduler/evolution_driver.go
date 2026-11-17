package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/submission"
	legacy "github.com/hatchet-dev/hatchet/pkg/client"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

const EvolutionTaskName = "pangu-evolution-case-v1"

type evolutionTaskInput struct {
	RequestID string `json:"request_id"`
}

func newEvolutionService() evolution.Service {
	s := &evolution.Service{Memory: learning.NewStore(), Entries: EvolutionEntries{}, Evidence: EvolutionEvidence{}}
	s.Memory = learning.NewStoreWithBenefit(s.CurrentBenefitExperimentTx)
	return *s
}
func (b Backend) EvolutionService() evolution.Service { return newEvolutionService() }

func NewEvolutionTask(client *hatchet.Client) *hatchet.StandaloneTask {
	return client.NewStandaloneTask(EvolutionTaskName, func(ctx hatchet.Context, in evolutionTaskInput) (map[string]string, error) {
		err := RunEvolutionCase(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), in.RequestID, ctx.WorkflowRunId())
		return map[string]string{"request_id": in.RequestID}, err
	}, hatchet.WithRetries(2), hatchet.WithExecutionTimeout(engineHorizon), hatchet.WithScheduleTimeout(engineHorizon), hatchet.WithWorkflowIdempotency(hatchet.IdempotencyConfig{Expression: "input.request_id", TTL: 90 * 24 * time.Hour, Method: hatchet.IdempotencyMethodTTL}))
}

// DispatchEvolution never sends the frozen checker or flow body to Hatchet input.
func (b Backend) DispatchEvolution(ctx context.Context, item *submission.Record) (string, error) {
	if item == nil {
		return "", evolution.ErrForbidden
	}
	return evolutionDispatch(ctx, b.Namespace(), item.ID, func(ctx context.Context, request string) (string, error) {
		ref, err := b.client.RunNoWait(ctx, EvolutionTaskName, evolutionTaskInput{RequestID: request}, hatchet.WithRunMetadata(map[string]string{"pangu_request_id": request, "pangu_purpose": "evaluation"}))
		var collision *legacy.IdempotencyViolationErr
		if errors.As(err, &collision) {
			return collision.ExistingRunExternalId, nil
		}
		if err != nil {
			return "", err
		}
		return ref.RunId, nil
	})
}

func (b Backend) DriveEvolution(ctx context.Context, scope evolution.Scope, id string, generation int) error {
	service := b.EvolutionService()
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var experimentID, state string
	err = conn.QueryRow(ctx, `SELECT experiment_id::text,state FROM pangu.evolution_intents WHERE id=$1 AND namespace=$2 AND project_id=$3 AND generation=$4`, id, scope.Namespace, scope.ProjectID, generation).Scan(&experimentID, &state)
	if err != nil {
		return err
	}
	if state != "evaluating" {
		return b.evolutionAdvanceIntent(ctx, scope, id, generation, state)
	}
	experiment, err := evaluation.ReadExperiment(ctx, scope.Namespace, scope.ProjectID, experimentID)
	if err != nil {
		return err
	}
	suite, err := evaluation.ReadExperimentSuite(ctx, scope.Namespace, experiment.SuiteID)
	if err != nil {
		return err
	}
	ready := true
	for _, c := range suite.Cases {
		if c.Partition == "development" {
			continue
		}
		for repeat := 1; repeat <= suite.Rules.Repeats; repeat++ {
			for _, arm := range []string{"baseline", "candidate"} {
				done, err := b.evolutionDriveCase(ctx, scope, evolution.CaseKey{IntentID: id, Generation: generation, Arm: arm, CaseID: c.ID, Repeat: repeat})
				if err != nil {
					return err
				}
				ready = ready && done
			}
		}
	}
	if !ready {
		return nil
	}
	decision, err := service.Decide(ctx, scope, id, generation)
	if err != nil {
		return err
	}
	if decision.Outcome != "passed" || experiment.Kind == "memory_benefit" {
		return nil
	}
	return b.evolutionAdvanceIntent(ctx, scope, id, generation, "passed")
}

func (b Backend) evolutionDriveCase(ctx context.Context, s evolution.Scope, key evolution.CaseKey) (bool, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	var recorded bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_executions e JOIN pangu.evolution_machine_results m ON m.request_id=e.request_id WHERE e.intent_id=$1 AND e.generation=$2 AND e.arm=$3 AND e.case_id=$4 AND e.repeat=$5)`, key.IntentID, key.Generation, key.Arm, key.CaseID, key.Repeat).Scan(&recorded)
	if err != nil || recorded {
		return recorded, err
	}
	service := b.EvolutionService()
	plan, err := service.PrepareCase(ctx, s, key)
	if err != nil {
		return false, err
	}
	if err = evolutionPersistPlan(ctx, s, *plan); err != nil {
		return false, err
	}
	record, err := submission.Dispatch(ctx, s.Namespace, plan.RequestID, b.DispatchEvolution)
	if err != nil {
		return false, err
	}
	var stopped bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pangu.evolution_case_runtime WHERE request_id=$1 AND status<>'running' AND stop_confirmed)`, plan.RequestID).Scan(&stopped)
	if err != nil {
		return false, err
	}
	if !stopped {
		stopped, err = b.evolutionRecoverCase(ctx, s, plan.RequestID, record.RunID)
		if err != nil || !stopped {
			return false, err
		}
	}
	err = service.RecordCaseResult(ctx, s, key, plan.RequestID)
	return err == nil, err
}

func (b Backend) evolutionAdvanceIntent(ctx context.Context, s evolution.Scope, id string, generation int, state string) error {
	service := b.EvolutionService()
	if state == "passed" {
		if _, err := service.Promote(ctx, s, id, generation); err != nil {
			return err
		}
		state = "switching"
	}
	if state != "switching" && state != "rolling_back" && state != "reconciling" && state != "observing" {
		return nil
	}
	if state == "reconciling" {
		unpublished, err := evolutionUnpublishedIntent(ctx, s, id, generation)
		if err != nil {
			return err
		}
		if unpublished {
			_, err = service.CancelEvaluation(ctx, s, id, generation, "恢复未发布实验的停止确认")
			return err
		}
	}
	current, err := service.Reconcile(ctx, s, id, generation)
	if err != nil {
		return err
	}
	remoteErr := b.evolutionIntentRemote(ctx, s, id, generation)
	current, err = service.Reconcile(ctx, s, id, generation)
	if err != nil {
		return errors.Join(remoteErr, err)
	}
	if current.State != "observing" {
		return remoteErr
	}
	observeErr := b.evolutionObserveRuns(ctx, s, id, generation)
	_, advanceErr := service.Advance(ctx, s, id, generation)
	return errors.Join(remoteErr, observeErr, advanceErr)
}
