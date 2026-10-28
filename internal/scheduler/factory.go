package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/submission"
)

// SyncFactory 只投影生成结果，不创建配置，也不执行候选中的任务。
func SyncFactory(ctx context.Context, ns string, s *factory.Session, source interface {
	GetRequest(context.Context, string) (*submission.Record, error)
	GetRun(context.Context, string) (*RunDetails, error)
}) error {
	for _, t := range s.Turns {
		if t.Status != "pending" {
			continue
		}
		record, err := source.GetRequest(ctx, t.RequestID)
		if err != nil {
			return err
		}
		if record.RunID == "" {
			continue
		}
		run, err := source.GetRun(ctx, record.RunID)
		if err != nil {
			return err
		}
		if !run.StopConfirmed {
			continue
		}
		raw, failure := "", ""
		if run.Status == "succeeded" {
			for _, node := range run.Nodes {
				if node.NodeID == "factory" {
					raw = node.Output
				}
			}
		} else {
			failure = "生成任务" + run.Status + "；请查看任务记录后重试"
			if run.Error != nil {
				failure += "：" + run.Error.Message
			}
		}
		if err = factory.Complete(ctx, ns, s.ID, t.ID, raw, failure); err != nil {
			return err
		}
	}
	return nil
}

func (b Backend) ReconcileFactory(ctx context.Context) error {
	ids, err := factory.Pending(ctx, b.Namespace())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		s, err := factory.Get(ctx, b.Namespace(), id)
		if err == nil {
			err = SyncFactory(ctx, b.Namespace(), s, b)
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
