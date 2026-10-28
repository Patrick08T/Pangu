package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

const cronTaskName = "pangu-cron-trigger-v1"

type cronInput struct {
	ScheduleID string `json:"schedule_id"`
	Revision   int    `json:"revision"`
}

func newCronTrigger(client *hatchet.Client) *hatchet.StandaloneTask {
	return client.NewStandaloneTask(cronTaskName, func(ctx hatchet.Context, input cronInput) (*schedule.Fire, error) {
		raw, err := getHatchetRun(ctx.GetContext(), ctx.WorkflowRunId())
		if err != nil {
			return nil, err
		}
		backend := NewBackend(client)
		namespace := client.NewWorkflow(WorkflowName).GetName()
		fire, err := schedule.Claim(ctx.GetContext(), namespace, input.ScheduleID, input.Revision, raw.Run.Metadata.CreatedAt, backend.requestActive)
		if err != nil || fire.Status != "pending" {
			return fire, err
		}
		return fire, backend.submitFire(ctx.GetContext(), fire)
	}, hatchet.WithRetries(2), hatchet.WithScheduleTimeout(time.Minute), hatchet.WithExecutionTimeout(30*time.Second))
}

func (b Backend) requestActive(ctx context.Context, id string) (bool, error) {
	request, err := b.GetRequest(ctx, id)
	if err != nil {
		return true, err
	}
	if request.RunID == "" {
		return true, nil
	}
	run, err := b.GetRun(ctx, request.RunID)
	if err != nil {
		return true, err
	}
	return !run.StopConfirmed, nil
}

func (b Backend) submitFire(ctx context.Context, fire *schedule.Fire) error {
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	key := fire.ScheduleID + ":" + fire.Slot.Format("20060102T1504Z")
	item, err := submission.Find(ctx, namespace, "cron", key, fire.Snapshot)
	if errors.Is(err, submission.ErrNotFound) {
		if err := fire.Snapshot.Workflow.ValidateHandoffs(); err != nil {
			return schedule.Skip(ctx, fire, err.Error())
		}
		item, err = submission.Prepare(ctx, namespace, "cron", key, fire.Snapshot)
	}
	if err != nil {
		return err
	}
	if err = schedule.Bind(ctx, fire, item.ID); err != nil {
		return err
	}
	_ = b.dispatch(ctx, item)
	return nil
}

func (b Backend) reconcileSchedules(ctx context.Context) error {
	fires, err := schedule.Pending(ctx, b.client.NewWorkflow(WorkflowName).GetName())
	if err != nil {
		return err
	}
	var failures []error
	for _, fire := range fires {
		if ctx.Err() != nil {
			break
		}
		if err := schedule.Checked(ctx, &fire); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := b.submitFire(ctx, &fire); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (b Backend) CronFires(ctx context.Context, id string, offset int64) ([]schedule.Fire, error) {
	return schedule.Fires(ctx, b.client.NewWorkflow(WorkflowName).GetName(), id, offset)
}

func (b Backend) CreateCron(ctx context.Context, c workflow.Cron) (*schedule.Plan, error) {
	if err := catalog.CheckDefinition(ctx, b.Namespace(), c.Workflow); err != nil {
		return nil, err
	}
	check, err := b.Preflight(ctx, c.Workflow)
	if err != nil {
		return nil, err
	}
	if !check.Ready {
		return nil, ErrNotReady
	}
	plan, err := schedule.Create(ctx, b.client.NewWorkflow(WorkflowName).GetName(), c)
	if err != nil {
		return nil, err
	}
	updated, err := schedule.ReconcileRemote(ctx, b.Namespace(), plan.ID, b.syncCron)
	if err != nil {
		slog.Warn("计划注册等待对账", "plan_id", plan.ID)
		return plan, nil
	}
	return updated, nil
}

func (b Backend) ListCrons(ctx context.Context, offset int64) ([]schedule.Plan, error) {
	return schedule.List(ctx, b.client.NewWorkflow(WorkflowName).GetName(), offset)
}

func (b Backend) GetCron(ctx context.Context, id string) (*schedule.Plan, error) {
	return schedule.Get(ctx, b.Namespace(), id)
}

func (b Backend) UpdateCron(ctx context.Context, id string, revision int, c workflow.Cron, paused bool) (*schedule.Plan, error) {
	current, err := schedule.Get(ctx, b.Namespace(), id)
	if err != nil {
		return nil, err
	}
	if !paused || !reflect.DeepEqual(current.Workflow, c.Workflow) {
		if err := catalog.CheckDefinition(ctx, b.Namespace(), c.Workflow); err != nil {
			return nil, err
		}
	}
	if !paused {
		check, err := b.Preflight(ctx, c.Workflow)
		if err != nil {
			return nil, err
		}
		if !check.Ready {
			return nil, ErrNotReady
		}
	}
	plan, err := schedule.Save(ctx, b.Namespace(), id, revision, c, paused)
	if err != nil {
		return nil, err
	}
	updated, err := schedule.ReconcileRemote(ctx, b.Namespace(), id, b.syncCron)
	if err != nil {
		slog.Warn("计划变更等待对账", "plan_id", id)
		return plan, nil
	}
	return updated, nil
}

func (b Backend) DeleteCron(ctx context.Context, id string) error {
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	if err := schedule.Delete(ctx, namespace, id); err != nil {
		return err
	}
	_, err := schedule.ReconcileRemote(ctx, namespace, id, b.syncCron)
	return err
}
