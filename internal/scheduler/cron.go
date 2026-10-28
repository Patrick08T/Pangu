package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"github.com/hatchet-dev/hatchet/sdks/go/features"
)

func (b Backend) ReconcileCrons(ctx context.Context) error {
	ids, err := schedule.RemotePending(ctx, b.Namespace())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		if _, err := schedule.ReconcileRemote(ctx, b.Namespace(), id, b.syncCron); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (b Backend) syncCron(ctx context.Context, plan *schedule.Plan) (string, error) {
	remote, err := b.findCron(ctx, "pangu-"+plan.ID)
	if err != nil {
		return "", err
	}
	if plan.Deleted || plan.Paused {
		if remote != nil {
			return "", b.deleteHatchetCron(ctx, remote.Metadata.Id)
		}
		return "", nil
	}
	if remote != nil {
		if remote.Cron == plan.Expression && remote.Enabled && remote.Input != nil && fmt.Sprint((*remote.Input)["revision"]) == fmt.Sprint(plan.Revision) {
			return remote.Metadata.Id, nil
		}
		if err := b.deleteHatchetCron(ctx, remote.Metadata.Id); err != nil {
			return "", err
		}
	}
	created, err := b.client.Crons().Create(ctx, b.client.NewWorkflow(cronTaskName).GetName(), features.CreateCronTrigger{Name: "pangu-" + plan.ID, Expression: plan.Expression, Input: map[string]any{"schedule_id": plan.ID, "revision": plan.Revision}})
	if err != nil {
		return "", err
	}
	if created == nil {
		return "", errors.New("Hatchet 未返回定时计划")
	}
	return created.Metadata.Id, nil
}

func (b Backend) findCron(ctx context.Context, name string) (*rest.CronWorkflows, error) {
	workflow := b.client.NewWorkflow(cronTaskName).GetName()
	var found *rest.CronWorkflows
	limit := int64(100)
	for offset := int64(0); ; offset += limit {
		page, err := b.client.Crons().List(ctx, rest.CronWorkflowListParams{CronName: &name, WorkflowName: &workflow, Limit: &limit, Offset: &offset})
		if err != nil {
			return nil, err
		}
		if page == nil || page.Rows == nil {
			return nil, errors.New("Hatchet 定时列表不可用")
		}
		for _, item := range *page.Rows {
			if item.Name == nil || *item.Name != name || item.WorkflowName != workflow {
				continue
			}
			if found != nil {
				return nil, errors.New("发现重复的远端计划，需要检查调度服务")
			}
			copy := item
			found = &copy
		}
		if len(*page.Rows) < int(limit) {
			return found, nil
		}
	}
}
