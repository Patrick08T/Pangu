package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

type Cancellation struct {
	RunID     string `json:"run_id"`
	Requested bool   `json:"requested"`
}

func (b Backend) CancelRun(ctx context.Context, id string) (*Cancellation, error) {
	details, err := b.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if details.StopConfirmed && (details.Status == "succeeded" || details.Status == "failed") {
		return &Cancellation{RunID: id}, nil
	}
	if err := execution.RequestCancel(ctx, b.client.NewWorkflow(WorkflowName).GetName(), id); err != nil {
		_, _ = b.cancelTree(ctx, id)
		return nil, err
	}
	if _, err := b.cancelTree(ctx, id); err != nil {
		slog.Warn("取消请求已持久保存，等待后台重试", "run_id", id)
	}
	return &Cancellation{RunID: id, Requested: true}, nil
}

func (b Backend) cancelTree(ctx context.Context, id string) (bool, error) {
	raw, err := b.ownedRun(ctx, id)
	if err != nil {
		return false, err
	}
	children, err := childRuns(ctx, raw)
	if err != nil {
		return false, err
	}
	ids := []uuid.UUID{}
	if !terminal(runStatus(raw.Run.Status)) {
		ids = append(ids, uuid.MustParse(id))
	}
	for _, node := range children {
		if !terminal(runStatus(node.Status)) {
			ids = append(ids, node.WorkflowRunExternalId)
		}
	}
	if len(ids) > 0 {
		_, err = b.client.Runs().Cancel(ctx, rest.V1CancelTaskRequest{ExternalIds: &ids})
		return false, err
	}
	active, err := execution.Active(ctx, id)
	if err != nil || active > 0 {
		return false, err
	}
	return true, execution.Reconciled(ctx, id)
}

func terminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled"
}

func (b Backend) Reconcile(ctx context.Context) error {
	if err := execution.Recover(ctx); err != nil {
		return err
	}
	ids, err := execution.Pending(ctx, b.client.NewWorkflow(WorkflowName).GetName())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if err := execution.Checked(ctx, id); err != nil {
			failures = append(failures, err)
			continue
		}
		if _, err := b.cancelTree(ctx, id); err != nil {
			failures = append(failures, fmt.Errorf("取消对账失败: %w", err))
		}
	}
	return errors.Join(failures...)
}

func (b Backend) Maintain(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := b.Reconcile(check); err != nil {
				slog.Warn("取消对账未完成，将重试")
			}
			cancel()
			check, cancel = context.WithTimeout(ctx, 10*time.Second)
			if err := b.ReconcileCrons(check); err != nil {
				slog.Warn("计划注册对账未完成，将重试")
			}
			cancel()
			check, cancel = context.WithTimeout(ctx, 10*time.Second)
			if err := b.reconcileSchedules(check); err != nil {
				slog.Warn("定时触发交接未完成，将重试")
			}
			if err := b.reconcileSubmissions(check); err != nil {
				slog.Warn("提交对账未完成，将重试")
			}
			cancel()
		}
	}
}
