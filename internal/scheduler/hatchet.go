package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

// shortcut: Hatchet 要求有限 duration；支持无期限执行后改用原生选项。
const engineHorizon time.Duration = 1<<63 - 1

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
	client.Events()
	return Backend{client: client}
}

func NewWorker(client *hatchet.Client, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error), inspect func(context.Context, workflow.Definition) workflow.Preflight) (*hatchet.Worker, error) {
	preflight := client.NewStandaloneTask(PreflightTaskName, func(ctx hatchet.Context, input preflightInput) (workflow.Preflight, error) {
		return inspectProject(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), input, inspect(ctx.GetContext(), input.Workflow)), nil
	}, hatchet.WithRetries(0), hatchet.WithExecutionTimeout(20*time.Second), hatchet.WithScheduleTimeout(8*time.Second))
	run := func(ctx hatchet.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		return runAgent(ctx, client.NewWorkflow(WorkflowName).GetName(), execute, input)
	}
	node := client.NewStandaloneTask(NodeTaskName, run,
		hatchet.WithRetries(2), hatchet.WithRetryBackoff(2, 30), hatchet.WithExecutionTimeout(engineHorizon), hatchet.WithScheduleTimeout(engineHorizon))
	codex := client.NewStandaloneTask(CodexTaskName, run,
		hatchet.WithRetries(2), hatchet.WithRetryBackoff(2, 30), hatchet.WithExecutionTimeout(engineHorizon), hatchet.WithScheduleTimeout(engineHorizon))
	root := client.NewStandaloneDurableTask(WorkflowName, func(ctx hatchet.DurableContext, input runInput) (map[string]workflow.NodeOutput, error) {
		runtime, err := loadRootExperience(ctx.GetContext(), client.NewWorkflow(WorkflowName).GetName(), ctx.WorkflowRunId(), input)
		if err != nil {
			return nil, hatchet.NewNonRetryableError(err)
		}
		d := runtime.request.Snapshot.Workflow
		if input.Fork != nil {
			if err := runtime.ValidateFork(ctx.GetContext()); err != nil {
				return nil, hatchet.NewNonRetryableError(err)
			}
		}
		if d.ProjectID != "" {
			if err := prepareWorkspace(ctx.GetContext(), ctx.WorkflowRunId(), client.NewWorkflow(WorkflowName).GetName(), input); err != nil {
				return nil, hatchet.NewNonRetryableError(err)
			}
		}
		var reused map[string]workflow.NodeOutput
		if input.Fork != nil {
			var err error
			reused, err = runtime.ImportFork(ctx.GetContext())
			if err != nil {
				return nil, hatchet.NewNonRetryableError(err)
			}
		}
		execute := func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
			return runtime.Execute(ctx.GetContext(), inputs, func(prepared []workflow.NodeInput) ([]workflow.NodeOutput, error) {
				return runLayer(ctx, client, node, codex, prepared)
			})
		}
		before := func(next *workflow.NodeInput, outputs map[string]workflow.NodeOutput) error {
			return waitApproval(ctx, client, input.RequestID, d, next, outputs, execute)
		}
		return runtime.Run(ctx.GetContext(), d, reused, before, execute)
	}, hatchet.WithRetries(2), hatchet.WithExecutionTimeout(engineHorizon), hatchet.WithScheduleTimeout(engineHorizon), hatchet.WithWorkflowIdempotency(hatchet.IdempotencyConfig{Expression: "input.request_id", TTL: 90 * 24 * time.Hour, Method: hatchet.IdempotencyMethodTTL}))
	cron := newCronTrigger(client)
	return client.NewWorker("pangu-worker", hatchet.WithWorkflows(node, codex, root, preflight, cron, newModelChoices(client), newPerforceTask(client), NewEvolutionTask(client)), hatchet.WithSlots(16), hatchet.WithDurableSlots(8))
}

func prepareWorkspace(ctx context.Context, id, namespace string, input runInput) error {
	if input.RequestID != "" {
		item, err := submission.Get(ctx, namespace, input.RequestID)
		if err != nil {
			return err
		}
		if item.Operation == "rebuild" {
			return workspace.EnsureRebuild(ctx, id, item.ProjectID, namespace, item.Snapshot.SourceRunID)
		}
	}
	if input.ApprovalOrigin != "" {
		cancelled, err := execution.CancelRequested(ctx, input.SourceRunID)
		if err != nil {
			return err
		}
		if cancelled {
			return execution.ErrCancelled
		}
	}
	if input.Fork != nil {
		return workspace.EnsureFork(ctx, id, input.Workflow.ProjectID, namespace, input.SourceRunID, input.Fork.Commit)
	}
	if input.SourceRunID != "" {
		return workspace.EnsureRerun(ctx, id, input.Workflow.ProjectID, namespace, input.SourceRunID)
	}
	return workspace.Ensure(ctx, id, input.Workflow.ProjectID, namespace)
}

func runLayer(ctx hatchet.DurableContext, client *hatchet.Client, eino, codex *hatchet.StandaloneTask, inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
	for _, input := range inputs {
		if err := validateNodeInput(input); err != nil {
			return nil, err
		}
	}
	jobs := make([]*nodeDispatch, 0, len(inputs))
	var failures []error
	for i, input := range inputs {
		job := &nodeDispatch{input: input, task: eino, name: NodeTaskName, namespace: client.NewWorkflow(WorkflowName).GetName(), index: i}
		if input.Node.Executor == "codex" {
			job.task, job.name = codex, CodexTaskName
		}
		// 保留 DurableContext；逐个派生避免整层输入超过单条 gRPC 消息限制。
		if err := dispatchNode(ctx, job); err != nil {
			failures = append(failures, err)
			break
		}
		jobs = append(jobs, job)
	}
	outputs := make([]workflow.NodeOutput, len(jobs))
	for _, job := range jobs {
		if err := collectNode(ctx, job, outputs); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return outputs, errors.Join(failures...)
	}
	if err := retryNodes(ctx, jobs, outputs); err != nil {
		return outputs, err
	}
	return outputs, finishDispatchedLayer(inputs, jobs, func() error {
		return resolveEvidence(ctx, client, jobs, outputs)
	})
}

func nodeRunIdentity(input workflow.NodeInput) (string, map[string]string) {
	key := input.Node.ID
	metadata := map[string]string{"pangu_node_id": input.Node.ID, "pangu_executor": input.Node.Executor}
	if input.ApprovalGeneration > 1 {
		key = fmt.Sprintf("%s-generation-%d", key, input.ApprovalGeneration)
		metadata["pangu_approval_generation"] = fmt.Sprint(input.ApprovalGeneration)
	}
	if it := input.Iteration; it != nil {
		key = fmt.Sprintf("%s-loop-%s-%d", key, it.ID, it.Round)
		metadata["pangu_loop_id"], metadata["pangu_loop_round"] = it.ID, fmt.Sprint(it.Round)
	}
	if f := input.ReviewFeedback; f != nil {
		key = fmt.Sprintf("%s-review-%s-%d", key, f.NodeID, f.Round)
		metadata["pangu_review_round"] = fmt.Sprint(f.Round)
		metadata["pangu_review_node"] = f.NodeID
	}
	if r := input.EvidenceRetry; r != nil {
		key = fmt.Sprintf("%s-evidence-%s", key, r.ApprovalID)
		metadata["pangu_evidence_round"] = fmt.Sprint(r.Round)
	}
	return key, metadata
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
	limit := int64(100)
	parent := run.Tasks[0].TaskExternalId
	includePayloads := true
	for offset := int64(0); ; offset += limit {
		response, err := api.V1WorkflowRunListWithResponse(ctx, tenant, &rest.V1WorkflowRunListParams{
			Since: run.Run.Metadata.CreatedAt.Add(-time.Second), Limit: &limit, Offset: &offset,
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
		if len(response.JSON200.Rows) < int(limit) {
			break
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
