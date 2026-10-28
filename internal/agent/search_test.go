package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestSearchPreflightUsesCodexWithoutSeparateKey(t *testing.T) {
	setupCodex(t)
	t.Setenv("OPENAI_API_KEY", "local-model")
	t.Setenv("PANGU_SEARCH_API_KEY", "")
	d := workflow.Definition{Input: `{"question":"查官方文档","public_queries":["OpenAI Codex 官方文档"]}`,
		Nodes: []workflow.Node{{ID: "pm", Executor: "eino", Model: "test", Role: "product_manager", Mode: "research"}}}
	if got := Preflight(context.Background(), d); !got.Ready {
		t.Fatalf("已登录 Codex 仍被额外搜索凭据阻止: %+v", got)
	}
	t.Setenv("PANGU_CODEX_BINARY", "missing")
	if got := Preflight(context.Background(), d); got.Ready {
		t.Fatal("搜索预检缓存了已失效的 Codex 配置")
	}
}

func TestCodexSearchOnlyAcceptsNativeResults(t *testing.T) {
	setupCodex(t)
	input := codexInput("search-fixture")
	input.ProjectID, input.ApprovalID, input.Node.ID = "private-project", "private-approval", "private-node"
	results, err := codexPublicSearch(context.Background(), input, "OpenAI Codex 官方文档")
	if err != nil || len(results) != 1 || results[0].URL != "https://developers.openai.com/codex/" {
		t.Fatalf("未使用原生搜索结果: %+v %v", results, err)
	}
	entries, err := os.ReadDir(os.Getenv("PANGU_WORKSPACE_ROOT"))
	if err != nil || len(entries) != 0 {
		t.Fatal("搜索临时目录未清理", err)
	}
}

func TestCodexSearchRejectsMissingOrWrongQueryEvidence(t *testing.T) {
	for _, raw := range []string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"https://fake.example"}}`,
		`{"type":"item.completed","item":{"type":"web_search","query":"private","action":{"type":"search","query":"private"},"results":[]}}`,
		`{"type":"item.completed","item":{"type":"web_search","query":"approved","action":{"type":"search","query":"approved"}}}`,
	} {
		s := &codexSearch{Query: "approved"}
		events := &codexEvents{Search: s}
		_, _ = events.Write([]byte(raw + "\n"))
		if _, err := s.results(); err == nil {
			t.Fatal("把模型回答、越权查询或缺失结果当成真实搜索")
		}
	}
}

func codexSearchHelper(input workflow.NodeInput, args map[string]string) int {
	if input.ProjectID != "" || input.ApprovalID != "" || input.Node.ID != "search" {
		return 54
	}
	if args["--sandbox"] != "read-only" || os.Getenv("PANGU_CODEX_TRACE") != "1" || os.Getenv("PANGU_CODEX_MODEL_ONLY") != "" || os.Getenv("PANGU_CODEX_SEARCH_QUERY") != input.Input || len(input.Parents) > 0 || len(input.Parameters) > 0 {
		return 51
	}
	if err := os.WriteFile(args["--output-last-message"], []byte("https://invented.example/"), 0600); err != nil {
		return 52
	}
	event := map[string]any{"type": "item.completed", "item": map[string]any{"type": "web_search", "query": input.Input,
		"action":  map[string]string{"type": "search", "query": input.Input},
		"results": []map[string]string{{"title": "Codex 文档", "url": "https://developers.openai.com/codex/"}}}}
	returnCode := 0
	if err := json.NewEncoder(os.Stdout).Encode(event); err != nil {
		returnCode = 53
	}
	return returnCode
}
