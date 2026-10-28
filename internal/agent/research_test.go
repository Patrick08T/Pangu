package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
		if len(tools) != 5 {
			t.Errorf("实际请求未注册工具: %d", len(tools))
		}
		args, _ := json.Marshal(map[string]any{"title": "任务台规划", "markdown": "## 结论\n建议 [input]\n## 证据与推断\n用户资料\n## 未知项\n待验证\n## PRD\n范围\n## 优先级与路线图\n先验证\n## 成功指标\n整理耗时", "source_ids": []string{"input"}})
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
