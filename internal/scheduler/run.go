package scheduler

import (
	"context"
	"errors"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"sort"

	"github.com/Patrick08T/Pangu/internal/execution"
)

type RunDetails struct {
	Fork *workflow.Fork `json:"fork,omitempty"`
	RunSummary
	Definition         *workflow.Definition `json:"definition,omitempty"`
	SourceRunID        string               `json:"source_run_id,omitempty"`
	Nodes              []NodeRun            `json:"nodes"`
	Events             []RunEvent           `json:"events"`
	Error              *RunError            `json:"error,omitempty"`
	CancelRequested    bool                 `json:"cancel_requested"`
	StopConfirmed      bool                 `json:"stop_confirmed"`
	Attempts           []execution.Attempt  `json:"attempts"`
	AttemptsNextOffset *int                 `json:"attempts_next_offset,omitempty"`
	LogsExpired        bool                 `json:"logs_expired"`
}

func (b Backend) GetRun(ctx context.Context, id string) (*RunDetails, error) {
	raw, err := b.ownedRun(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return b.archivedRun(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	children, err := childRuns(ctx, raw)
	if err != nil {
		return nil, err
	}
	run := raw.Run
	out := &RunDetails{RunSummary: RunSummary{ID: run.Metadata.Id, Status: runStatus(run.Status),
		CreatedAt: run.Metadata.CreatedAt, StartedAt: optionalTime(run.StartedAt), FinishedAt: optionalTime(run.FinishedAt)},
		Nodes: []NodeRun{}, Events: []RunEvent{}}
	record, lookupErr := submission.ForRun(ctx, b.Namespace(), id)
	if lookupErr != nil && !errors.Is(lookupErr, submission.ErrNotFound) {
		return nil, lookupErr
	}
	if record != nil {
		out.ProjectID = record.ProjectID
		out.CreatedBy = record.CreatedBy
		out.Definition = &record.Snapshot.Workflow
		out.SourceRunID = record.Snapshot.SourceRunID
		out.CreatedAt = record.CreatedAt
	}
	out.Error = runError(run.ErrorMessage)
	for _, row := range children {
		out.Nodes = append(out.Nodes, nodeRun(row))
	}
	enrichBranches(out, record)
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].NodeID < out.Nodes[j].NodeID })
	for _, row := range raw.TaskEvents {
		out.Events = append(out.Events, runEvent(row))
	}
	if err := enrichExecution(ctx, out); err != nil {
		return nil, err
	}
	if !out.LogsExpired && out.Definition != nil {
		out.Title = taskTitle(*out.Definition)
	}
	return out, b.saveHistory(ctx, out)
}

func enrichExecution(ctx context.Context, out *RunDetails) error {
	requested, err := execution.CancelRequested(ctx, out.ID)
	if err != nil {
		return err
	}
	out.CancelRequested = requested
	page, err := execution.ListAttempts(ctx, out.ID, 0, 20)
	if err != nil {
		return err
	}
	out.Attempts, out.AttemptsNextOffset = page.Items, page.NextOffset
	if err := enrichErrors(ctx, out); err != nil {
		return err
	}
	stopped := terminal(out.Status)
	for _, node := range out.Nodes {
		stopped = stopped && terminal(node.Status)
	}
	active, err := execution.Active(ctx, out.ID)
	if err != nil {
		return err
	}
	stopped = stopped && active == 0
	out.StopConfirmed = stopped
	if (requested || out.Status == "cancelled") && !stopped {
		out.Status = "cancelling"
	}
	return redactExpired(ctx, out)
}

func redactExpired(ctx context.Context, out *RunDetails) error {
	expired, err := workspace.Expired(ctx, out.ID)
	if err != nil {
		return err
	}
	out.LogsExpired = expired
	if expired {
		out.Title = ""
		out.Definition = nil
		out.Fork = nil
		out.Events = []RunEvent{}
		out.Attempts = []execution.Attempt{}
		out.AttemptsNextOffset = nil
		if out.Error != nil {
			out.Error.Message = "日志已超过保留期"
		}
		for i := range out.Nodes {
			out.Nodes[i].Output = ""
			if out.Nodes[i].Error != nil {
				out.Nodes[i].Error.Message = "日志已超过保留期"
			}
		}
	}
	return nil
}

func enrichErrors(ctx context.Context, out *RunDetails) error {
	categories, err := execution.LatestCategories(ctx, out.ID)
	if err != nil {
		return err
	}
	for i := range out.Nodes {
		node := &out.Nodes[i]
		if node.Status == "succeeded" {
			node.Error = nil
			continue
		}
		if category := categories[node.ID]; node.Error != nil && category != "" {
			node.Error.Category = category
		}
	}
	return nil
}

func (b Backend) ListRuns(ctx context.Context, q RunQuery) (*RunList, error) {
	ids, err := submission.RunIDs(ctx, b.Namespace(), q.ProjectID, q.Since, q.Until, q.Offset, q.Limit+1)
	if err != nil {
		return nil, err
	}
	out := &RunList{Items: []RunSummary{}, Since: q.Since, Until: q.Until}
	if int64(len(ids)) > q.Limit {
		next := q.Offset + q.Limit
		out.NextOffset, ids = &next, ids[:q.Limit]
	}
	for _, id := range ids {
		run, err := b.runSummary(ctx, id)
		if err != nil {
			return nil, err
		}
		if q.Status == "" || q.Status == run.Status {
			out.Items = append(out.Items, *run)
		}
	}
	return out, nil
}
