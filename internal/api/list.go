package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

type page[T any] struct {
	Items      []T    `json:"items"`
	NextOffset *int64 `json:"next_offset,omitempty"`
}

func (h handler) cronPage(ctx context.Context, items []schedule.Plan, offset int64, project string) (page[schedule.Plan], error) {
	out := page[schedule.Plan]{Items: []schedule.Plan{}}
	if len(items) == 100 {
		next := offset + 100
		out.NextOffset = &next
	}
	for _, item := range items {
		if project != "" && project != item.Workflow.ProjectID {
			continue
		}
		ok, err := h.visible(ctx, item.Workflow.ProjectID, "read")
		if err != nil {
			return out, err
		}
		if ok {
			out.Items = append(out.Items, item)
		}
	}
	return out, nil
}

func (h handler) projectList(ctx context.Context, items []workspace.Project) ([]workspace.Project, error) {
	out := []workspace.Project{}
	for _, item := range items {
		ok, err := h.visible(ctx, item.ID, "read")
		if err != nil {
			return nil, err
		}
		if ok {
			admin, err := h.visible(ctx, "", "admin")
			if err != nil {
				return nil, err
			}
			if !admin {
				item.Source = ""
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func (h handler) runList(ctx context.Context, out *scheduler.RunList) error {
	items := []scheduler.RunSummary{}
	for _, item := range out.Items {
		ok, err := h.visible(ctx, item.ProjectID, "read")
		if err != nil {
			return err
		}
		if ok {
			items = append(items, item)
		}
	}
	out.Items = items
	return nil
}
