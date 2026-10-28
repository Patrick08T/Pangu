package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func nativeSearchWorkspace(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	dir := nativePolicyWorkspace(t, handler)
	t.Setenv("PANGU_WORKSPACE_ROOT", dir)
	path := filepath.Join(os.Getenv("PANGU_CODEX_HOME"), "config.toml")
	config, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config = append([]byte("model=\"gpt-5.4\"\n"), config...)
	config = append(config, []byte("supports_standalone_web_search=true\n")...)
	if err = os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNativeSearchForcesAuthorizedQueryAndReturnsToolSources(t *testing.T) {
	var modelCalls, searches atomic.Int32
	const query = "OpenAI Codex 官方文档"
	nativeSearchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/alpha/search") {
			searches.Add(1)
			var body struct{ Commands, Input json.RawMessage }
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				t.Error("搜索请求无效")
			}
			if strings.Contains(string(body.Input), "private-") {
				t.Error("搜索发送了内部项目元数据")
			}
			var commands map[string]any
			_ = json.Unmarshal(body.Commands, &commands)
			queries, ok := commands["search_query"].([]any)
			if !ok || len(queries) != 1 || queries[0].(map[string]any)["q"] != query || commands["open"] != nil {
				t.Error("搜索扩大了授权", string(body.Commands))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"output":"真实工具结果","results":[{"url":"https://developers.openai.com/codex/","title":"Codex"}]}`)
			return
		}
		var request map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("模型请求无效")
		}
		if strings.Contains(string(request["input"]), "private-") {
			t.Error("搜索模型发送了内部项目元数据")
		}
		if modelCalls.Add(1) == 1 {
			nativeToolResponse(w, map[string]any{"type": "function_call", "call_id": "search-1", "namespace": "web", "name": "run", "arguments": `{"search_query":[{"q":"not authorized"}],"open":[{"ref_id":"https://private.example"}]}`})
			return
		}
		nativeToolResponse(w, map[string]any{"type": "message", "id": "done", "role": "assistant", "content": []map[string]string{{"type": "output_text", "text": `{"done":true}`}}})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results, err := codexPublicSearch(ctx, workflow.NodeInput{ProjectID: "private-project", ApprovalID: "private-approval", Node: workflow.Node{ID: "private-node"}}, query)
	if err != nil || len(results) != 1 || results[0].URL != "https://developers.openai.com/codex/" || searches.Load() != 1 || modelCalls.Load() != 2 {
		t.Fatalf("原生搜索未按授权执行: %+v %v search=%d model=%d", results, err, searches.Load(), modelCalls.Load())
	}
}

func TestNativeSearchRejectsLocalTools(t *testing.T) {
	dir := nativeSearchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		nativeToolResponse(w, map[string]any{"type": "function_call", "call_id": "shell", "name": "exec_command", "arguments": `{"cmd":"echo forbidden"}`})
	})
	input := workflow.NodeInput{Workspace: dir, Node: workflow.Node{ID: "search", Executor: "codex", ReadOnly: true, Policy: &workflow.Policy{}}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var events []trace.Event
	ctx = trace.WithSink(ctx, func(_ context.Context, event trace.Event) error { events = append(events, event); return nil })
	_, err := runCodex(ctx, input, "", &codexSearch{Query: "approved"})
	if err == nil || !strings.Contains(err.Error(), "PANGU_POLICY_SEARCH_ONLY") {
		t.Fatal("搜索会话允许本机工具", err)
	}
	for _, event := range events {
		if event.Kind == "tool" && event.Phase == "end" && event.Status == "failed" {
			return
		}
	}
	t.Fatal("缺少被权限拒绝的工具调用记录")
}

func TestNativeCodexSearchLiveOnce(t *testing.T) {
	if os.Getenv("PANGU_CODEX_SEARCH_REAL") != "1" {
		t.Skip("需要明确授权一次真实原生搜索")
	}
	dir, err := os.MkdirTemp(os.Getenv("PANGU_WORKSPACE_ROOT"), "search-smoke-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	schemaPath := filepath.Join(dir, "response.json")
	if err = os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"done":{"type":"boolean"}},"required":["done"],"additionalProperties":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	search := &codexSearch{Query: "OpenAI Codex 官方文档"}
	input := workflow.NodeInput{Workspace: dir, Input: search.Query, Node: workflow.Node{ID: "search", Executor: "codex", ReadOnly: true,
		Instruction: "只调用一次 web.run 搜索 input 中的词，然后返回 done=true。不要读取或修改文件，不要使用其他工具，不重试。",
		Policy:      &workflow.Policy{}}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = runCodex(ctx, input, schemaPath, search)
	if len(search.raw) > 0 {
		_ = os.WriteFile(filepath.Join("..", "..", ".artifacts", "codex-search-live-results.json"), search.raw, 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
	results, err := search.results()
	if err != nil || len(results) == 0 {
		t.Fatalf("真实搜索未返回来源: %v", err)
	}
	valid := false
	for _, item := range results {
		t.Logf("来源: %s %s", item.Title, item.URL)
		address, parseErr := url.Parse(item.URL)
		valid = valid || parseErr == nil && address.Scheme == "https" && (address.Hostname() == "openai.com" || strings.HasSuffix(address.Hostname(), ".openai.com"))
	}
	if !valid {
		t.Fatal("搜索没有返回可登记的 OpenAI 官方来源")
	}
}
