package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

func (b Backend) cancelEvolutionConsumer(ctx context.Context, scope evolution.Scope, item evolutionConsumer) error {
	if !item.evaluation {
		_, err := b.cancelTree(ctx, item.run)
		return err
	}
	// Evaluator runs use their own engine workflow and cannot pass ownedRun's
	// business-workflow check. Validate that identity before any remote mutation.
	raw, err := getHatchetRun(ctx, item.run)
	if err != nil {
		return err
	}
	expected, err := registeredWorkflowID(ctx, b.client.NewWorkflow(EvolutionTaskName).GetName())
	if err != nil {
		return err
	}
	if raw.Run.WorkflowId != expected || raw.Run.ParentTaskExternalId != nil {
		return evolution.ErrConflict
	}
	running := !terminal(runStatus(raw.Run.Status))
	for _, task := range raw.Tasks {
		running = running || !terminal(runStatus(task.Status))
	}
	if running {
		run, err := uuid.Parse(item.run)
		if err != nil {
			return err
		}
		ids := []uuid.UUID{run}
		_, err = b.client.Runs().Cancel(ctx, rest.V1CancelTaskRequest{ExternalIds: &ids})
		return err
	}
	stopped, err := b.evolutionRecoverCase(ctx, scope, item.request, item.run)
	if err != nil || !stopped {
		return err
	}
	return execution.Reconciled(ctx, item.run)
}
