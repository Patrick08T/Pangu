package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func TestCodexChatTurnKeepsToolsInEinoAndPreservesHistory(t *testing.T) {
	setupCodex(t)
	input := codexInput("bridge")
	messages := []*schema.Message{schema.SystemMessage("中文规划"), schema.UserMessage("用户主题")}
	tools := []*schema.ToolInfo{{Name: "read_project", Desc: "读取已授权资料", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"document_index": {Type: schema.Integer, Required: true}})}}
	got, err := codexTurn(context.Background(), input, messages, tools)
	if err != nil || len(got.ToolCalls) != 1 || got.ToolCalls[0].Function.Name != "read_project" {
		t.Fatalf("工具桥接失败: %+v %v", got, err)
	}
	messages = append(messages, got, schema.ToolMessage("资料内容", got.ToolCalls[0].ID))
	got, err = codexTurn(context.Background(), input, messages, tools)
	if err != nil || got.Content != "已根据资料完成" || len(got.ToolCalls) != 0 {
		t.Fatalf("工具结果未传回模型: %+v %v", got, err)
	}
	entries, err := os.ReadDir(os.Getenv("PANGU_WORKSPACE_ROOT"))
	if err != nil || len(entries) != 0 {
		t.Fatal("桥接临时目录未清理", err)
	}
}

func TestCodexReplyRejectsInvalidToolsAndPayloads(t *testing.T) {
	tools := []*schema.ToolInfo{{Name: "read_project"}}
	for _, text := range []string{
		`{"content":"","tool_calls":[{"name":"shell","arguments":"{}"}]}`,
		`{"content":"","tool_calls":[{"name":"read_project","arguments":"[]"}]}`,
		`{"content":"","tool_calls":[{"name":"read_project","arguments":"{} {}"}]}`,
		`{"content":"ok","tool_calls":[],"unexpected":true}`,
		`{"content":"ok","tool_calls":[]} {}`,
		`{"content":"","tool_calls":[]}`,
		strings.Repeat("x", 16385),
	} {
		if _, err := codexReply(text, tools); err == nil {
			t.Fatalf("接受无效模型输出: %s", text[:min(len(text), 100)])
		}
	}
}

func TestCodexBridgeAllowsReportArgumentsAboveNodeSummaryLimit(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"markdown": strings.Repeat("调研证据", 3000)})
	reply, _ := json.Marshal(map[string]any{"content": "", "tool_calls": []schema.FunctionCall{{Name: "register_report", Arguments: string(args)}}})
	if _, err := codexReply(string(reply), []*schema.ToolInfo{{Name: "register_report"}}); err != nil {
		t.Fatal("内部报告工具参数被 16 KiB 节点摘要限制截断", err)
	}
	reply, _ = json.Marshal(map[string]any{"content": strings.Repeat("x", 1024*1024+1), "tool_calls": []any{}})
	if _, err := codexReply(string(reply), nil); err == nil {
		t.Fatal("未限制桥接回复大小")
	}
}

func TestCodexModelDoesNotSilentlyIgnoreOptions(t *testing.T) {
	m := &codexModel{}
	if _, err := m.Generate(context.Background(), nil, model.WithMaxTokens(20)); err == nil {
		t.Fatal("忽略了不支持的选项")
	}
	bound, err := m.WithTools([]*schema.ToolInfo{{Name: "read_project"}})
	if err != nil || len(m.tools) != 0 || len(bound.(*codexModel).tools) != 1 {
		t.Fatal("WithTools 修改了原对象")
	}
}

func codexConnectionInput(t *testing.T, modelName string) (workflow.NodeInput, string, string) {
	t.Helper()
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ns := uuid.NewString()
	p, err := workspace.Register(ctx, ns, workspace.Project{Name: "Codex 桥接测试", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "connection", Name: "Codex", Spec: catalog.Spec{Connection: &catalog.Connection{Provider: "codex", Model: modelName}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return workflow.NodeInput{ProjectID: p.ID, ProcessScope: uuid.NewString(), Node: workflow.Node{ID: "pm", Executor: "eino", Model: modelName, ConnectionVersion: v.ID, Instruction: "中文规划"}}, ns, item.ID
}

func TestEinoCodexAccountModelRechecksConnectionBeforeEveryTurn(t *testing.T) {
	setupCodex(t)
	input, ns, id := codexConnectionInput(t, "bridge")
	ctx := context.Background()
	chat, err := einoModel(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	tools := []*schema.ToolInfo{{Name: "read_project", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}}
	if _, err = chat.Generate(ctx, []*schema.Message{schema.SystemMessage("中文规划"), schema.UserMessage("用户主题")}, model.WithTools(tools)); err != nil {
		t.Fatal(err)
	}
	check := Preflight(ctx, workflow.Definition{ProjectID: input.ProjectID, Nodes: []workflow.Node{input.Node}})
	if !check.Ready || check.Checks[0].Status != "ready" {
		t.Fatal("账号连接预检失败", check)
	}
	if err = catalog.Archive(ctx, ns, id, true); err != nil {
		t.Fatal(err)
	}
	if _, err = chat.Generate(ctx, []*schema.Message{schema.UserMessage("继续")}, model.WithTools(tools)); err == nil {
		t.Fatal("模型缓存了已经归档的配置")
	}
	if Preflight(ctx, workflow.Definition{ProjectID: input.ProjectID, Nodes: []workflow.Node{input.Node}}).Ready {
		t.Fatal("预检没有重新读取归档状态")
	}
}

func TestNativeCodexEinoProductManager(t *testing.T) {
	if os.Getenv("PANGU_CODEX_BRIDGE_REAL") != "1" {
		t.Skip("需要明确启用真实 Codex 账号调用")
	}
	input, _, _ := codexConnectionInput(t, "")
	input.Node.Role, input.Node.Mode = "product_manager", "planning"
	input.Node.Policy = &workflow.Policy{}
	input.Input = `{"question":"为一个任务清单原型提出最小产品规划","audience":"个人开发者","market":"中国","time_scope":"当前","context":"只需要新增、完成两个操作；这是联调样例。","constraints":"仅依据本输入，不搜索，不读取文件，所有效果均标记为待验证。"}`
	result, err := Execute(context.Background(), input)
	if err != nil || len(result.Reports) != 1 || len(result.ReportIDs) != 1 || len(result.Evidence) == 0 {
		t.Fatalf("真实 Eino→Codex→产品经理工具失败: %+v %v", result, err)
	}
	t.Log("真实 Codex 账号已通过 Eino 调用 register_report；生成一份中文规划报告")
}

func codexBridgeHelper(inputJSON []byte, args map[string]string) int {
	var input struct {
		Input string `json:"input"`
	}
	if json.Unmarshal(inputJSON, &input) != nil || args["--sandbox"] != "read-only" || os.Getenv("PANGU_CODEX_MODEL_ONLY") != "1" {
		return 31
	}
	if _, err := os.ReadFile(args["--output-schema"]); err != nil {
		return 32
	}
	var prompt struct {
		Messages []*schema.Message `json:"messages"`
		Tools    []any             `json:"tools"`
	}
	if json.Unmarshal([]byte(input.Input), &prompt) != nil || len(prompt.Tools) != 1 || len(prompt.Messages) < 2 || prompt.Messages[0].Content != "中文规划" {
		return 33
	}
	result := `{"content":"","tool_calls":[{"name":"read_project","arguments":"{\"document_index\":0}"}]}`
	if prompt.Messages[len(prompt.Messages)-1].Role == schema.Tool {
		if prompt.Messages[len(prompt.Messages)-1].Content != "资料内容" {
			return 34
		}
		result = `{"content":"已根据资料完成","tool_calls":[]}`
	}
	if os.WriteFile(args["--output-last-message"], []byte(result), 0600) != nil {
		return 35
	}
	return 0
}
