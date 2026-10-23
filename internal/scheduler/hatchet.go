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
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/hatchet-dev/hatchet/sdks/go/features"
)

const WorkflowName = "pangu-orchestrate-v1"
const NodeTaskName = "pangu-agent-v1"

type Backend struct{ client *hatchet.Client }

var ErrNotFound = errors.New("运行不存在或尚未同步，请稍后重试")
var ErrNotReady = errors.New("工作流尚未注册，请先启动 Pangu Worker")

func NewBackend(client *hatchet.Client) Backend {
	// SDK 的功能客户端采用无锁延迟初始化，在 HTTP 并发请求前完成初始化。
	client.Runs()
	client.Crons()
	return Backend{client: client}
}

type RunDetails struct {
	Run   *rest.V1WorkflowRunDetails `json:"run"`
	Nodes []rest.V1TaskSummary       `json:"nodes"`
}

func NewWorker(client *hatchet.Client, execute func(context.Context, workflow.NodeInput) (workflow.NodeOutput, error)) (*hatchet.Worker, error) {
	node := client.NewStandaloneTask(NodeTaskName, func(ctx hatchet.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
		out, err := execute(ctx.GetContext(), input)
		out.RunID = ctx.WorkflowRunId()
		return out, err
	}, hatchet.WithRetries(2), hatchet.WithRetryBackoff(2, 30), hatchet.WithExecutionTimeout(3*time.Minute))
	root := client.NewStandaloneDurableTask(WorkflowName, func(ctx hatchet.DurableContext, d workflow.Definition) (map[string]workflow.NodeOutput, error) {
		return workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
			return runLayer(ctx, node, inputs)
		})
	}, hatchet.WithRetries(2), hatchet.WithExecutionTimeout(6*time.Hour))
	return client.NewWorker("pangu-worker", hatchet.WithWorkflows(node, root), hatchet.WithSlots(16), hatchet.WithDurableSlots(8))
}

func runLayer(ctx hatchet.DurableContext, task *hatchet.StandaloneTask, inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
	for _, input := range inputs {
		if err := validateNodeInput(input); err != nil {
			return nil, err
		}
	}
	refs := make([]*hatchet.WorkflowRunRef, len(inputs))
	for i, input := range inputs {
		// 保留 DurableContext；逐个派生避免整层输入超过单条 gRPC 消息限制。
		ref, err := task.RunNoWait(ctx, input, hatchet.WithRunKey(input.Node.ID),
			hatchet.WithRunMetadata(map[string]string{"pangu_node_id": input.Node.ID}))
		if err != nil {
			return nil, err
		}
		refs[i] = ref
	}
	outputs := make([]workflow.NodeOutput, len(refs))
	for i := range refs {
		result, err := refs[i].Result()
		if err != nil {
			return nil, fmt.Errorf("节点 %s: %w", inputs[i].Node.ID, err)
		}
		if err := result.TaskOutput(NodeTaskName).Into(&outputs[i]); err != nil {
			return nil, err
		}
		outputs[i].RunID = refs[i].RunId
	}
	return outputs, nil
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

func (b Backend) Submit(ctx context.Context, d workflow.Definition) (string, error) {
	if err := b.checkRegistered(ctx); err != nil {
		return "", err
	}
	ref, err := b.client.RunNoWait(ctx, WorkflowName, d)
	if err != nil {
		return "", err
	}
	return ref.RunId, nil
}

func (b Backend) checkRegistered(ctx context.Context) error {
	api, tenant, err := restClient()
	if err != nil {
		return err
	}
	name := b.client.NewWorkflow(WorkflowName).GetName()
	response, err := api.WorkflowListWithResponse(ctx, tenant, &rest.WorkflowListParams{Name: &name})
	if err != nil {
		return err
	}
	if response.JSON200 == nil {
		return fmt.Errorf("查询工作流注册失败: HTTP %d", response.StatusCode())
	}
	if response.JSON200.Rows != nil {
		for _, row := range *response.JSON200.Rows {
			if row.Name == name {
				return nil
			}
		}
	}
	return ErrNotReady
}

func (b Backend) GetRun(ctx context.Context, id string) (*RunDetails, error) {
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
	run := response.JSON200
	out := &RunDetails{Run: run, Nodes: []rest.V1TaskSummary{}}
	if len(run.Tasks) == 0 {
		return out, nil
	}
	limit := int64(33)
	parent := run.Tasks[0].TaskExternalId
	includePayloads := true
	nodes, err := b.client.Runs().List(ctx, rest.V1WorkflowRunListParams{
		Since: run.Run.Metadata.CreatedAt.Add(-time.Second), Limit: &limit,
		ParentTaskExternalId: &parent, IncludePayloads: &includePayloads,
	})
	if err != nil {
		return nil, err
	}
	// Hatchet 的 OnlyTasks 路径忽略父任务筛选，使用工作流查询并仅保留直接子任务。
	for _, node := range nodes.Rows {
		if node.ParentTaskExternalId != nil && *node.ParentTaskExternalId == parent {
			out.Nodes = append(out.Nodes, node)
		}
	}
	return out, nil
}

func (b Backend) CreateCron(ctx context.Context, c workflow.Cron) (*rest.CronWorkflows, error) {
	data, err := json.Marshal(c.Workflow)
	if err != nil {
		return nil, err
	}
	var input map[string]interface{}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	return b.client.Crons().Create(ctx, b.client.NewWorkflow(WorkflowName).GetName(), features.CreateCronTrigger{Name: c.Name, Expression: c.Expression, Input: input})
}

func (b Backend) ListCrons(ctx context.Context, offset int64) (*rest.CronWorkflowsList, error) {
	name, limit := b.client.NewWorkflow(WorkflowName).GetName(), int64(100)
	out, err := b.client.Crons().List(ctx, rest.CronWorkflowListParams{WorkflowName: &name, Offset: &offset, Limit: &limit})
	if err == nil && out == nil {
		return nil, fmt.Errorf("Hatchet 未返回 Cron 列表")
	}
	return out, err
}

func (b Backend) DeleteCron(ctx context.Context, id string) error {
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
	if response.StatusCode() != http.StatusNoContent {
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
