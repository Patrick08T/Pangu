package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestEinoProductManagerActuallyInvokesRegisteredTools(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		var tools []any
		_ = json.Unmarshal(body["tools"], &tools)
		if len(tools) != 1 {
			t.Errorf("实际请求未注册工具: %d", len(tools))
		}
		args, _ := json.Marshal(map[string]any{"title": "任务台规划", "markdown": "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时", "source_ids": []string{"input"}, "claims": []research.Claim{{ID: "c1", Statement: "效果待验证", Kind: "unknown", Scope: "当前团队", Uncertainty: "尚无真实用户证据"}}})
		encoded, _ := json.Marshal(string(args))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"tool-1","type":"function","function":{"name":"register_report","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, encoded)
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	brief := workflow.ResearchBrief{Question: "规划任务台", Audience: "研发团队", Market: "中国", TimeScope: "2026", Context: "目前手工记录", Constraints: "预算未知"}
	raw, _ := json.Marshal(brief)
	input := workflow.NodeInput{Node: workflow.Node{ID: "pm", Executor: "eino", Model: "test", Instruction: "中文规划", Role: "product_manager", Mode: "planning"}, Input: string(raw)}
	result, err := Execute(context.Background(), input)
	if err != nil || len(result.ReportIDs) != 1 || len(result.Reports) != 1 || len(result.Evidence) != 1 || result.Evidence[0].ExitCode != 0 || calls != 1 {
		t.Fatalf("工具没有实际执行: %+v %v calls=%d", result, err, calls)
	}
}

func TestEinoRepairsReportValidationFailureInSameAttempt(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Messages []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		body := "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时"
		if calls == 1 {
			body = "错误格式"
		}
		if calls == 2 && !strings.Contains(request.Messages[len(request.Messages)-1].Content, "二级标题") {
			t.Error("校验错误未回传模型")
		}
		if calls > 2 {
			t.Error("登记成功后又调用模型")
		}
		args, _ := json.Marshal(map[string]any{"title": "规划", "markdown": body, "source_ids": []string{"input"}, "claims": []research.Claim{{ID: "c1", Statement: "建议待验证", Kind: "unknown", Scope: "团队", Uncertainty: "尚无用户数据"}}})
		encoded, _ := json.Marshal(string(args))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"repair","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-%d","type":"function","function":{"name":"register_report","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, calls, encoded)
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	in := workflow.NodeInput{Node: workflow.Node{ID: "pm", Executor: "eino", Model: "test", Instruction: "中文规划", Role: "product_manager", Mode: "planning", Policy: &workflow.Policy{}}, Input: `{"question":"规划任务台"}`}
	out, err := Execute(context.Background(), in)
	if err != nil || len(out.Reports) != 1 || calls != 2 || len(out.Evidence) != 2 {
		t.Fatalf("未修正报告: calls=%d evidence=%v err=%v", calls, out.Evidence, err)
	}
}

func TestEinoStopsRemainingBatchToolsAfterReport(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		args, _ := json.Marshal(map[string]any{"title": "规划", "markdown": "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时", "source_ids": []string{"input"}, "claims": []research.Claim{{ID: "c1", Statement: "建议待验证", Kind: "unknown", Scope: "团队", Uncertainty: "尚无用户数据"}}})
		encoded, _ := json.Marshal(string(args))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"batch","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"first","type":"function","function":{"name":"register_report","arguments":%s}},{"id":"second","type":"function","function":{"name":"register_report","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, encoded, encoded)
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	in := workflow.NodeInput{Node: workflow.Node{ID: "pm", Executor: "eino", Model: "test", Instruction: "中文规划", Role: "product_manager", Mode: "planning", Policy: &workflow.Policy{}}, Input: `{"question":"规划任务台"}`}
	out, err := Execute(context.Background(), in)
	if err != nil || len(out.Reports) != 1 || calls != 1 || len(out.Evidence) != 1 {
		t.Fatalf("已登记后仍执行同批工具: calls=%d evidence=%v err=%v", calls, out.Evidence, err)
	}
}
