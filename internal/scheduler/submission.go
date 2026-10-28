package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	legacy "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type runInput struct {
	RequestID string `json:"request_id"`
	submission.Snapshot
}

func (b Backend) Submit(ctx context.Context, d workflow.Definition, key string) (*submission.Record, error) {
	if err := d.ValidateInput(); err != nil {
		return nil, err
	}
	if _, err := d.Layers(); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(d.ProjectID); err != nil {
		return nil, errors.New("提交必须指定已登记的 project_id")
	}
	if !submission.ValidKey(key) {
		return nil, errors.New("Idempotency-Key 必须为 1–128 个字母、数字、点、冒号、下划线或连字符")
	}
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	snapshot := submission.Snapshot{Workflow: d}
	item, err := submission.Find(ctx, namespace, "submit", key, snapshot)
	if err != nil && !errors.Is(err, submission.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, submission.ErrNotFound) {
		if err := catalog.CheckDefinition(ctx, namespace, d); err != nil {
			return nil, err
		}
		check, err := b.Preflight(ctx, d)
		if err != nil {
			return nil, err
		}
		if !check.Ready {
			return nil, ErrNotReady
		}
		item, err = submission.Prepare(ctx, namespace, "submit", key, snapshot)
		if err != nil {
			return nil, err
		}
	}
	return b.dispatch(ctx, item), nil
}

func (b Backend) dispatch(ctx context.Context, item *submission.Record) *submission.Record {
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	out, err := submission.Dispatch(ctx, namespace, item.ID, b.sendSubmission)
	if err != nil {
		slog.Warn("提交意图已持久化，等待对账", "request_id", item.ID)
		return item
	}
	return out
}

func (b Backend) GetRequest(ctx context.Context, id string) (*submission.Record, error) {
	return submission.Get(ctx, b.client.NewWorkflow(WorkflowName).GetName(), id)
}

func (b Backend) sendSubmission(ctx context.Context, item *submission.Record) (string, error) {
	workflowID, err := b.registeredID(ctx)
	if err != nil {
		return "", err
	}
	api, tenant, err := restClient()
	if err != nil {
		return "", err
	}
	ids := []string{"pangu_request_id:" + item.ID}
	workflows := []uuid.UUID{workflowID}
	limit := int64(2)
	response, err := api.V1WorkflowRunListWithResponse(ctx, tenant, &rest.V1WorkflowRunListParams{Since: item.CreatedAt.Add(-time.Minute), AdditionalMetadata: &ids, Limit: &limit, WorkflowIds: &workflows})
	if err != nil {
		return "", err
	}
	if response.JSON200 == nil {
		return "", errors.New("提交对账查询失败")
	}
	if len(response.JSON200.Rows) > 1 {
		return "", errors.New("提交键对应多个运行，需要人工核查")
	}
	if len(response.JSON200.Rows) == 1 {
		return response.JSON200.Rows[0].WorkflowRunExternalId.String(), nil
	}
	ref, err := b.client.RunNoWait(ctx, WorkflowName, runInput{RequestID: item.ID, Snapshot: item.Snapshot}, hatchet.WithRunMetadata(map[string]string{"pangu_request_id": item.ID, "pangu_project_id": item.ProjectID, "pangu_created_by": item.CreatedBy}))
	var collision *legacy.IdempotencyViolationErr
	if errors.As(err, &collision) {
		return collision.ExistingRunExternalId, nil
	}
	if err != nil {
		return "", err
	}
	return ref.RunId, nil
}

func (b Backend) reconcileSubmissions(ctx context.Context) error {
	namespace := b.client.NewWorkflow(WorkflowName).GetName()
	ids, err := submission.Pending(ctx, namespace)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if _, err := submission.Dispatch(ctx, namespace, id, b.sendSubmission); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
