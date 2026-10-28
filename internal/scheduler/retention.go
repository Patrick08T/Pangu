package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/delivery"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"log/slog"
	"time"
)

func (b Backend) Retention(ctx context.Context, preview bool) ([]workspace.RetentionItem, error) {
	items, err := workspace.RetentionCandidates(ctx, b.Namespace())
	if err != nil {
		return nil, err
	}
	for i := range items {
		item := &items[i]
		item.Status = "protected"
		if item.ExpiredAt == nil {
			run, err := b.GetRun(ctx, item.RunID)
			if err != nil || !run.StopConfirmed || run.FinishedAt == nil || run.FinishedAt.After(time.Now().Add(-30*24*time.Hour)) {
				if !preview {
					if err := workspace.RetentionChecked(ctx, item.RunID, item.Status); err != nil {
						return nil, err
					}
				}
				continue
			}
			item.FinishedAt = run.FinishedAt
		}
		item.Status = "eligible"
		if preview {
			continue
		}
		if item.FinishedAt == nil {
			return nil, errors.New("清理记录缺少终结时间")
		}
		err := workspace.Expire(ctx, item.RunID, *item.FinishedAt)
		switch {
		case err == nil:
			item.Status = "purged"
		case errors.Is(err, workspace.ErrLeased):
			item.Status = "leased"
		default:
			item.Status = "failed"
		}
		if err := workspace.RetentionChecked(ctx, item.RunID, item.Status); err != nil {
			return nil, err
		}
	}
	if !preview {
		err = errors.Join(factory.Maintain(ctx, b.Namespace()), workspace.PruneHistory(ctx, b.Namespace()), evaluation.Maintain(ctx, b.Namespace()), delivery.Maintain(ctx, b.Namespace()), usage.Maintain(ctx))
	}
	return items, err
}

func (b Backend) MaintainRetention(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 5*time.Minute)
			items, err := b.Retention(check, false)
			cancel()
			if err != nil {
				slog.Warn("保留清理未完成，将重试")
				continue
			}
			for _, item := range items {
				slog.Info("保留清理", "run_id", item.RunID, "status", item.Status)
			}
		}
	}
}
