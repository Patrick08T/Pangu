package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/execution"
	"time"
)

func waitNode(ctx context.Context, run, node string) error {
	for {
		ready, err := execution.DispatchReady(ctx, run, node)
		if err != nil || ready {
			return err
		}
		if err = debugWait(ctx); err != nil {
			return err
		}
	}
}

func beginAttempt(ctx context.Context, item *execution.Attempt) error {
	for {
		err := execution.Begin(ctx, item)
		if !errors.Is(err, execution.ErrPaused) {
			return err
		}
		if err = debugWait(ctx); err != nil {
			return err
		}
	}
}

func debugWait(ctx context.Context) error {
	// 只读持久状态，不改变 Hatchet durable 调用序列；重放仍定位同一个子运行。
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
