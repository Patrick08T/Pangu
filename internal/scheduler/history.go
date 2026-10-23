package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
)

func (b Backend) saveHistory(ctx context.Context, run *RunDetails) error {
	if run.ProjectID == "" || !run.StopConfirmed || run.FinishedAt == nil {
		return nil
	}
	summary := RunDetails{RunSummary: run.RunSummary, SourceRunID: run.SourceRunID, StopConfirmed: true,
		CancelRequested: run.CancelRequested, LogsExpired: true, Nodes: []NodeRun{}, Events: []RunEvent{}, Attempts: []execution.Attempt{}}
	summary.Error = historyError(run.Error)
	for _, node := range run.Nodes {
		summary.Nodes = append(summary.Nodes, NodeRun{RunSummary: node.RunSummary, NodeID: node.NodeID, TaskID: node.TaskID,
			Executor: node.Executor, Attempt: node.Attempt, Error: historyError(node.Error)})
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return submission.SaveHistory(ctx, b.Namespace(), run.ID, *run.FinishedAt, body)
}

func historyError(value *RunError) *RunError {
	if value == nil {
		return nil
	}
	return &RunError{Category: value.Category, Message: "日志已超过保留期"}
}

func (b Backend) archivedRun(ctx context.Context, id string) (*RunDetails, error) {
	body, err := submission.History(ctx, b.Namespace(), id)
	if errors.Is(err, submission.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var run RunDetails
	if err := json.Unmarshal(body, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (b Backend) ArchiveRuns(ctx context.Context) error {
	ids, err := submission.HistoryPending(ctx, b.Namespace())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		if _, err := b.GetRun(ctx, id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (b Backend) runSummary(ctx context.Context, id string) (*RunSummary, error) {
	run, err := b.archivedRun(ctx, id)
	if errors.Is(err, ErrNotFound) {
		run, err = b.GetRun(ctx, id)
	}
	if errors.Is(err, ErrNotFound) {
		record, lookupErr := submission.ForRun(ctx, b.Namespace(), id)
		if lookupErr != nil {
			return nil, lookupErr
		}
		return &RunSummary{ID: record.RunID, ProjectID: record.ProjectID, CreatedBy: record.CreatedBy, CreatedAt: record.CreatedAt, Status: "unknown"}, nil
	}
	if err != nil {
		return nil, err
	}
	return &run.RunSummary, nil
}

func (b Backend) MaintainHistory(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		check, cancel := context.WithTimeout(ctx, 50*time.Second)
		err := b.ArchiveRuns(check)
		cancel()
		if err != nil {
			slog.Warn("运行终态摘要归档未完成，将重试")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
