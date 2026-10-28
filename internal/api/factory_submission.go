package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func (h handler) generateFactory(ctx context.Context, s *factory.Session, in factory.Input) (*submission.Record, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	// 网络重试必须复用已保存快照，不因新上下文而产生第二次模型调用。
	for _, turn := range s.Turns {
		if turn.ID == in.ID {
			if turn.Input != in {
				return nil, catalog.ErrConflict
			}
			return h.backend.GetRequest(ctx, turn.RequestID)
		}
	}
	data, err := h.factoryContext(ctx, s.ProjectID, in)
	if err != nil {
		return nil, err
	}
	d, digest, err := factory.Definition(*s, in, *data)
	if err != nil {
		return nil, err
	}
	if err = catalog.CheckDefinition(ctx, h.backend.Namespace(), d); err != nil {
		return nil, err
	}
	ready, err := h.backend.Preflight(ctx, d)
	if err != nil {
		return nil, err
	}
	if !ready.Ready {
		return nil, scheduler.ErrNotReady
	}
	record, err := factory.Prepare(ctx, h.backend.Namespace(), s.ID, in, d, digest)
	if err != nil {
		return nil, err
	}
	accepted, err := h.backend.Submit(ctx, d, factory.Key(in.ID))
	if err == nil {
		return accepted, nil
	}
	return record, nil
}

func (h handler) factoryContext(ctx context.Context, project string, in factory.Input) (*factory.Context, error) {
	ns := h.backend.Namespace()
	choices, err := catalog.ConnectionChoices(ctx, ns, 0)
	if err != nil {
		return nil, err
	}
	found := false
	for _, c := range choices {
		found = found || c.ID == in.ConnectionVersion
	}
	if !found {
		return nil, fmt.Errorf("%w: 请选择已发布的共享模型连接", catalog.ErrInvalid)
	}
	out := &factory.Context{Connections: choices}
	if in.TargetID != "" {
		item, err := catalog.Get(ctx, ns, in.TargetID)
		if err != nil {
			return nil, err
		}
		if item.Archived || (item.Kind != "agent" && item.Kind != "workflow") {
			return nil, fmt.Errorf("%w: 目标必须是工作空间未归档的 Agent 或工作流", catalog.ErrInvalid)
		}
		out.Target = item
		if item.Revision != in.TargetRevision {
			return nil, catalog.ErrConflict
		}
		if item.Spec.Flow != nil {
			for _, step := range item.Spec.Flow.Nodes {
				v, err := catalog.ReadVersion(ctx, ns, step.AgentVersion)
				if err != nil {
					return nil, err
				}
				out.Components = append(out.Components, *v)
			}
		}
	}
	if in.SourceRunID != "" {
		run, err := h.backend.GetRun(ctx, in.SourceRunID)
		if err != nil {
			return nil, err
		}
		if run.ProjectID != project || run.LogsExpired || !run.StopConfirmed {
			return nil, fmt.Errorf("%w: 请选择当前项目已结束且未过期的任务", catalog.ErrInvalid)
		}
		data, err := json.Marshal(struct {
			ID, Status string
			Nodes      []scheduler.NodeRun
			Error      *scheduler.RunError
		}{run.ID, run.Status, run.Nodes, run.Error})
		if err != nil {
			return nil, err
		}
		if len(data) > 48000 {
			return nil, fmt.Errorf("%w: 所选任务证据超过 48 KiB，请选择更小的任务", catalog.ErrInvalid)
		}
		out.Evidence = string(data)
	}
	return out, nil
}
