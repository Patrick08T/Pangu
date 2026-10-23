package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

const WorkflowName = "pangu-orchestrate-v2"
const NodeTaskName = "pangu-agent-v2"
const CodexTaskName = "pangu-codex-v1"

type Backend struct{ client *hatchet.Client }

func (b Backend) Namespace() string { return b.client.NewWorkflow(WorkflowName).GetName() }

var ErrNotFound = errors.New("运行不存在或尚未同步，请稍后重试")
var ErrNotReady = errors.New("执行环境尚未就绪，请检查编排注册、在线 Worker 和所需执行器配置")

func NewBackend(client *hatchet.Client) Backend {
	// SDK 的功能客户端采用无锁延迟初始化，在 HTTP 并发请求前完成初始化。
	client.Runs()
	client.Crons()
	return Backend{client: client}
}

func NewWorker(client *hatchet.Client, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error), inspect func(context.Context, workflow.Definition) workflow.Preflight) (*hatchet.Worker, error) {
	preflight := client.NewStandaloneTask(PreflightTaskName, func(ctx hatchet.Context, input preflightInput) (workflow.Preflight, error) {
		return inspectProject(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), input, inspect(ctx.GetContext(), input.Workflow)), nil
	}, hatchet.WithRetries(0), hatchet.WithExecutionTimeout(8*time.Second), hatchet.WithScheduleTimeout(8*time.Second))
	run := func(ctx hatchet.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		return runAgent(ctx, execute, input)
	}
	node := client.NewStandaloneTask(NodeTaskName, run,
		hatchet.WithRetries(2), hatchet.WithRetryBackoff(2, 30), hatchet.WithExecutionTimeout(3*time.Minute))
	maximum, strategy := int32(1), hatchet.GroupRoundRobin
	codex := client.NewStandaloneTask(CodexTaskName, run,
		hatchet.WithRetries(2), hatchet.WithRetryBackoff(2, 30), hatchet.WithExecutionTimeout(30*time.Minute),
		hatchet.WithScheduleTimeout(6*time.Hour),
		hatchet.WithConcurrency(&hatchet.Concurrency{Expression: "'codex-workspace'", MaxRuns: &maximum, LimitStrategy: &strategy}))
	root := client.NewStandaloneDurableTask(WorkflowName, func(ctx hatchet.DurableContext, input runInput) (map[string]workflow.NodeOutput, error) {
		d := input.Workflow
		if d.ProjectID != "" {
			if err := prepareWorkspace(ctx.GetContext(), ctx.WorkflowRunId(), client.NewWorkflow(WorkflowName).GetName(), input); err != nil {
				return nil, hatchet.NewNonRetryableError(err)
			}
		}
		return workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
			return runLayer(ctx, node, codex, inputs)
		})
	}, hatchet.WithRetries(2), hatchet.WithExecutionTimeout(6*time.Hour), hatchet.WithWorkflowIdempotency(hatchet.IdempotencyConfig{Expression: "input.request_id", TTL: 90 * 24 * time.Hour, Method: hatchet.IdempotencyMethodTTL}))
	cron := newCronTrigger(client)
	return client.NewWorker("pangu-worker", hatchet.WithWorkflows(node, codex, root, preflight, cron), hatchet.WithSlots(16), hatchet.WithDurableSlots(8))
}

func prepareWorkspace(ctx context.Context, id, namespace string, input runInput) error {
	if input.SourceRunID != "" {
		return workspace.EnsureRerun(ctx, id, input.Workflow.ProjectID, namespace, input.SourceRunID)
	}
	return workspace.Ensure(ctx, id, input.Workflow.ProjectID, namespace)
}

func runLayer(ctx hatchet.DurableContext, eino, codex *hatchet.StandaloneTask, inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
	for _, input := range inputs {
		if err := validateNodeInput(input); err != nil {
			return nil, err
		}
	}
	refs := make([]*hatchet.WorkflowRunRef, 0, len(inputs))
	names := make([]string, len(inputs))
	var failures []error
	for i, input := range inputs {
		if err := allowDispatch(ctx.GetContext(), ctx.WorkflowRunId()); err != nil {
			failures = append(failures, err)
			break
		}
		task := eino
		names[i] = NodeTaskName
		if input.Node.Executor == "codex" {
			task, names[i] = codex, CodexTaskName
		}
		// 保留 DurableContext；逐个派生避免整层输入超过单条 gRPC 消息限制。
		ref, err := task.RunNoWait(ctx, input, hatchet.WithRunKey(input.Node.ID),
			hatchet.WithRunMetadata(map[string]string{"pangu_node_id": input.Node.ID, "pangu_executor": input.Node.Executor}))
		if err != nil {
			failures = append(failures, err)
			break
		}
		refs = append(refs, ref)
	}
	outputs := make([]workflow.NodeOutput, len(refs))
	for i := range refs {
		result, err := refs[i].Result()
		if err != nil {
			failures = append(failures, fmt.Errorf("节点 %s: %w", inputs[i].Node.ID, err))
			continue
		}
		if err := result.TaskOutput(names[i]).Into(&outputs[i]); err != nil {
			failures = append(failures, err)
		}
		outputs[i].RunID = refs[i].RunId
	}
	return outputs, errors.Join(failures...)
}

func validateNodeInput(input workflow.NodeInput) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if len(data) > 1024*1024 {
		return fmt.Errorf("节点 %s 的序列化输入超过 1 MiB", input.Node.ID)
	}
	return nil
}

func (b Backend) checkRegistered(ctx context.Context) error {
	_, err := b.registeredID(ctx)
	return err
}

func (b Backend) registeredID(ctx context.Context) (uuid.UUID, error) {
	return registeredWorkflowID(ctx, b.client.NewWorkflow(WorkflowName).GetName())
}

func registeredWorkflowID(ctx context.Context, name string) (uuid.UUID, error) {
	api, tenant, err := restClient()
	if err != nil {
		return uuid.Nil, err
	}
	limit := 100
	for offset := 0; ; offset += limit {
		response, err := api.WorkflowListWithResponse(ctx, tenant, &rest.WorkflowListParams{Name: &name, Limit: &limit, Offset: &offset})
		if err != nil {
			return uuid.Nil, err
		}
		if response.JSON200 == nil {
			return uuid.Nil, fmt.Errorf("查询工作流注册失败: HTTP %d", response.StatusCode())
		}
		if response.JSON200.Rows == nil {
			return uuid.Nil, ErrNotReady
		}
		for _, row := range *response.JSON200.Rows {
			if row.Name == name {
				return uuid.Parse(row.Metadata.Id)
			}
		}
		if len(*response.JSON200.Rows) < limit {
			return uuid.Nil, ErrNotReady
		}
	}
}

// Check 只验证 Hatchet 连接与编排注册，不承诺 Worker、模型和项目目录就绪。
func (b Backend) Check(ctx context.Context) error { return b.checkRegistered(ctx) }

func (b Backend) ownedRun(ctx context.Context, id string) (*rest.V1WorkflowRunDetails, error) {
	workflowID, err := b.registeredID(ctx)
	if err != nil {
		return nil, err
	}
	run, err := getHatchetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.Run.WorkflowId != workflowID || run.Run.ParentTaskExternalId != nil {
		return nil, ErrNotFound
	}
	return run, nil
}

func getHatchetRun(ctx context.Context, id string) (*rest.V1WorkflowRunDetails, error) {
	api, _, err := restClient()
	if err != nil {
		return nil, err
	}
	runID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	response, err := api.V1WorkflowRunGetWithResponse(ctx, runID, nil)
	if err != nil {
		return nil, err
	}
	if response.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("查询运行失败: HTTP %d", response.StatusCode())
	}
	return response.JSON200, nil
}

func childRuns(ctx context.Context, run *rest.V1WorkflowRunDetails) ([]rest.V1TaskSummary, error) {
	out := []rest.V1TaskSummary{}
	if len(run.Tasks) == 0 {
		return out, nil
	}
	api, tenant, err := restClient()
	if err != nil {
		return nil, err
	}
	limit := int64(33)
	parent := run.Tasks[0].TaskExternalId
	includePayloads := true
	response, err := api.V1WorkflowRunListWithResponse(ctx, tenant, &rest.V1WorkflowRunListParams{
		Since: run.Run.Metadata.CreatedAt.Add(-time.Second), Limit: &limit,
		ParentTaskExternalId: &parent, IncludePayloads: &includePayloads,
	})
	if err != nil {
		return nil, err
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("查询子节点失败: HTTP %d", response.StatusCode())
	}
	// Hatchet 的 OnlyTasks 路径忽略父任务筛选，使用工作流查询并仅保留直接子任务。
	for _, node := range response.JSON200.Rows {
		if node.ParentTaskExternalId != nil && *node.ParentTaskExternalId == parent {
			out = append(out, node)
		}
	}
	return out, nil
}

func (b Backend) deleteHatchetCron(ctx context.Context, id string) error {
	api, tenant, err := restClient()
	if err != nil {
		return err
	}
	cronID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	// 使用生成的 REST 客户端，按删除接口定义校验 204 响应。
	response, err := api.WorkflowCronDeleteWithResponse(ctx, tenant, cronID)
	if err != nil {
		return err
	}
	if response.StatusCode() != http.StatusNoContent && response.StatusCode() != http.StatusNotFound {
		return fmt.Errorf("删除 Cron 失败: HTTP %d", response.StatusCode())
	}
	return nil
}

func restClient() (*rest.ClientWithResponses, uuid.UUID, error) {
	tenant, err := uuid.Parse(os.Getenv("HATCHET_CLIENT_TENANT_ID"))
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("HATCHET_CLIENT_TENANT_ID 不合法: %w", err)
	}
	server, token := os.Getenv("HATCHET_CLIENT_SERVER_URL"), os.Getenv("HATCHET_CLIENT_TOKEN")
	if server == "" || token == "" {
		return nil, uuid.Nil, fmt.Errorf("缺少 Hatchet REST 地址或令牌")
	}
	api, err := rest.NewClientWithResponses(server, rest.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+token)
		return nil
	}))
	return api, tenant, err
}
