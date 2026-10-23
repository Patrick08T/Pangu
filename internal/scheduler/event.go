package scheduler

import (
	"context"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/workspace"

	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

type EventPage struct {
	Items      []RunEvent `json:"items"`
	NextOffset *int       `json:"next_offset,omitempty"`
}

func (b Backend) Events(ctx context.Context, root, node string, offset, limit int) (*EventPage, error) {
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("事件分页无效")
	}
	raw, err := b.ownedRun(ctx, root)
	if errors.Is(err, ErrNotFound) {
		return b.expiredEvents(ctx, root, node)
	}
	if err != nil {
		return nil, err
	}
	if expired, err := workspace.Expired(ctx, root); err != nil {
		return nil, err
	} else if expired {
		return nil, workspace.ErrExpired
	}
	if len(raw.Tasks) == 0 {
		return &EventPage{Items: []RunEvent{}}, nil
	}
	task := raw.Tasks[0].TaskExternalId
	if node != "" {
		children, err := childRuns(ctx, raw)
		if err != nil {
			return nil, err
		}
		task = uuid.Nil
		for _, child := range children {
			if child.WorkflowRunExternalId.String() == node {
				task = child.TaskExternalId
				break
			}
		}
		if task == uuid.Nil {
			return nil, ErrNotFound
		}
	}
	return taskEvents(ctx, task, offset, limit)
}

func (b Backend) expiredEvents(ctx context.Context, root, node string) (*EventPage, error) {
	run, err := b.archivedRun(ctx, root)
	if err != nil {
		return nil, err
	}
	if node != "" {
		found := false
		for _, item := range run.Nodes {
			found = found || item.ID == node
		}
		if !found {
			return nil, ErrNotFound
		}
	}
	return nil, workspace.ErrExpired
}

func taskEvents(ctx context.Context, task uuid.UUID, offset, limit int) (*EventPage, error) {
	api, _, err := restClient()
	if err != nil {
		return nil, err
	}
	start, count := int64(offset), int64(limit+1)
	response, err := api.V1TaskEventListWithResponse(ctx, task, &rest.V1TaskEventListParams{Offset: &start, Limit: &count})
	if err != nil {
		return nil, err
	}
	if response.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("查询调度事件失败: HTTP %d", response.StatusCode())
	}
	out := &EventPage{Items: []RunEvent{}}
	if response.JSON200.Rows != nil {
		for _, row := range *response.JSON200.Rows {
			if row.TaskId != task {
				return nil, fmt.Errorf("Hatchet 返回了其他任务的事件")
			}
			out.Items = append(out.Items, runEvent(row))
		}
	}
	if len(out.Items) > limit {
		next := offset + limit
		out.NextOffset = &next
		out.Items = out.Items[:limit]
	}
	return out, nil
}
