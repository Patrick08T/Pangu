package scheduler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/artifact"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetProductManagerFourModesAndReportChain(t *testing.T) {
	_, client := integrationClient(t)
	model := httptest.NewServer(http.HandlerFunc(pmModel(t)))
	defer model.Close()
	t.Setenv("OPENAI_BASE_URL", model.URL)
	t.Setenv("OPENAI_API_KEY", "test-model-key")
	worker, err := scheduler.NewWorker(client, agent.Execute, agent.Preflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	project := registerTestProject(t, backend)
	brief := workflow.ResearchBrief{Question: "规划任务台", Audience: "小型团队", Market: "中国", TimeScope: "2026", Context: "目前手工登记", Constraints: "工期未知", Documents: []string{"README.md"}, Competitors: []string{"示例产品"}, Dimensions: []string{"用户", "功能"}}
	raw, _ := json.Marshal(brief)
	d := workflow.Definition{ProjectID: project.ID, Input: string(raw)}
	previous := ""
	for _, mode := range []string{"research", "competition", "planning", "design"} {
		node := workflow.Node{ID: mode, Executor: "eino", Model: mode, Role: "product_manager", Mode: mode, Handoff: testHandoff(), Instruction: "根据已授权资料生成中文报告"}
		if previous != "" {
			node.DependsOn = []string{previous}
		}
		d.Nodes = append(d.Nodes, node)
		previous = mode
	}
	accepted, err := backend.Submit(context.Background(), d, "pm-four-modes")
	if err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, api.New(backend, integrationAccess{}), accepted.RunID)
	if run.Status != "succeeded" || len(run.Nodes) != 4 || len(run.Attempts) != 4 {
		t.Fatalf("四能力链路失败: %+v", run)
	}
	manifest, err := artifact.List(context.Background(), run.ID)
	if err != nil || len(manifest.Files) != 4 {
		t.Fatalf("报告产物缺失: %+v %v", manifest, err)
	}
	for _, attempt := range run.Attempts {
		var full execution.Attempt
		request(t, api.New(backend, integrationAccess{}), "GET", fmt.Sprintf("/runs/%s/nodes/%s/attempts/%d", run.ID, attempt.NodeRunID, attempt.Attempt), nil, 200, &full)
		if len(full.Evidence) < 2 {
			t.Fatalf("缺少工具调用证据: %+v", full)
		}
	}
	for _, file := range manifest.Files {
		_, data, _, err := artifact.Read(context.Background(), run.ID, file.ID, false)
		if err != nil || !strings.Contains(string(data), "获取时间") {
			t.Fatalf("来源证据缺失: %v", err)
		}
	}
}

func pmModel(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		toolResults := 0
		parent := ""
		for _, message := range body.Messages {
			if message.Role == "tool" {
				toolResults++
				if strings.Contains(message.Content, `"error"`) {
					t.Errorf("工具失败: %s", message.Content)
				}
			}
			if message.Role == "user" {
				var input struct {
					Parents map[string]workflow.NodeOutput `json:"parents"`
				}
				_ = json.Unmarshal([]byte(message.Content), &input)
				for id := range input.Parents {
					parent = id
				}
			}
		}
		name, args := "register_report", pmReport(body.Model, parent)
		if toolResults == 0 {
			name, args = "read_project", map[string]any{"document_index": 0}
		} else if parent != "" && toolResults == 1 {
			name, args = "read_parent_report", map[string]any{"parent_id": parent, "page": 0}
		}
		raw, _ := json.Marshal(args)
		encoded, _ := json.Marshal(string(raw))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"tool-%d","type":"function","function":{"name":"%s","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, toolResults, name, encoded)
	}
}

func pmReport(mode, parent string) map[string]any {
	sections := map[string][]string{"research": {"调研范围与方法", "发现与建议"}, "competition": {"竞品矩阵", "定位差异"}, "planning": {"PRD", "优先级与路线图", "成功指标"}, "design": {"用户流程", "输入输出与异常边界", "任务与依赖", "验收条件"}}
	body := "## 结论\n建议继续验证 [d0]\n## 证据与推断\n基于项目资料作推断\n## 未知项\n真实用户效果待验证"
	for _, section := range sections[mode] {
		body += "\n## " + section + "\n模拟验证内容；不作为真实模型质量验收"
	}
	sources := []string{"d0"}
	if parent != "" {
		sources = append(sources, "p"+parent+"_0")
	}
	result := map[string]any{"title": mode + " 中文报告", "markdown": body, "source_ids": sources}
	result["claims"] = []research.Claim{{ID: "c1", Statement: "基准资料标识为验证项目", Kind: "fact", Scope: "当前测试 Git 基准", Uncertainty: "模拟测试不代表真人采纳", Evidence: []research.Evidence{{SourceID: "d0", Quote: "验证项目", SourceDate: "unknown"}}}}
	if mode == "design" {
		result["tasks"] = []research.Task{{ID: "api", Title: "实现 API", Scope: "输入校验", Deliverable: "接口", Acceptance: "错误输入返回 400"}, {ID: "ui", Title: "实现界面", Scope: "提交表单", DependsOn: []string{"api"}, Deliverable: "页面", Acceptance: "可查看结果"}}
	}
	return result
}
