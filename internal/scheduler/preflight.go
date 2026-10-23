package scheduler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

const PreflightTaskName = "pangu-preflight-v1"

type preflightInput struct {
	Workflow    workflow.Definition `json:"workflow"`
	SourceRunID string              `json:"source_run_id,omitempty"`
}

func (b Backend) Preflight(ctx context.Context, definition workflow.Definition) (*workflow.Preflight, error) {
	return b.preflight(ctx, preflightInput{Workflow: definition})
}

func (b Backend) preflight(ctx context.Context, input preflightInput) (*workflow.Preflight, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := b.registeredID(ctx); err != nil {
		return nil, err
	}
	online, err := b.onlineWorker(ctx)
	if err != nil {
		return nil, err
	}
	if !online {
		return &workflow.Preflight{Checks: []workflow.Check{{Component: "worker", Status: "unavailable"}}}, nil
	}
	result, err := b.client.Run(ctx, PreflightTaskName, input)
	if err != nil {
		return nil, errors.Join(ErrNotReady, errors.New("Worker 诊断未在期限内完成"))
	}
	var out workflow.Preflight
	if err := result.TaskOutput(PreflightTaskName).Into(&out); err != nil {
		return nil, err
	}
	out.Checks = append([]workflow.Check{{Component: "worker", Status: "ready"}}, out.Checks...)
	return &out, nil
}

func (b Backend) onlineWorker(ctx context.Context) (bool, error) {
	api, tenant, err := restClient()
	if err != nil {
		return false, err
	}
	name, limit := b.client.NewWorkflow(WorkflowName).GetName(), int64(100)
	statuses := []rest.WorkerStatus{rest.ACTIVE}
	for offset := int64(0); ; offset += limit {
		response, err := api.WorkerListWithResponse(ctx, tenant, &rest.WorkerListParams{Offset: &offset, Limit: &limit, Statuses: &statuses})
		if err != nil {
			return false, err
		}
		if response.JSON200 == nil || response.JSON200.Rows == nil {
			return false, errors.New("无法获取在线 Worker 列表")
		}
		for _, item := range *response.JSON200.Rows {
			if workerSupports(item, name, time.Now()) {
				return true, nil
			}
		}
		if len(*response.JSON200.Rows) < int(limit) {
			return false, nil
		}
	}
}

func workerSupports(worker rest.Worker, name string, now time.Time) bool {
	if worker.Status == nil || *worker.Status != rest.ACTIVE || worker.LastHeartbeatAt == nil ||
		now.Sub(*worker.LastHeartbeatAt) > 30*time.Second || worker.Actions == nil {
		return false
	}
	root, inspect := false, false
	preflight := strings.TrimSuffix(name, WorkflowName) + PreflightTaskName
	for _, action := range *worker.Actions {
		root = root || action == name+":"+WorkflowName
		inspect = inspect || action == preflight+":"+PreflightTaskName
	}
	return root && inspect
}
