package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func TestResponsesModelContinuesToolCall(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var input struct {
			Model     string
			Store     *bool
			Include   []string
			Input     []map[string]any
			Reasoning struct{ Effort string }
			Tools     []map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer fixture" ||
			input.Model != "gpt-6.2-sol" || input.Reasoning.Effort != "high" ||
			input.Store == nil || *input.Store || len(input.Include) != 1 || input.Include[0] != "reasoning.encrypted_content" ||
			len(input.Tools) != 1 || input.Tools[0]["name"] != "read_sample" {
			t.Errorf("请求缺少模型、工具或隐私配置: %+v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Write([]byte(responseToolFixture))
			return
		}
		if len(input.Input) != 5 || input.Input[2]["encrypted_content"] != "opaque" ||
			input.Input[3]["call_id"] != "call_a" || input.Input[4]["call_id"] != "call_a" || input.Input[4]["output"] != "已核查" {
			t.Errorf("未依序回传推理及工具结果: %+v", input.Input)
		}
		w.Write([]byte(responseTextFixture))
	}))
	defer server.Close()
	chat := newResponsesModel(&openai.ChatModelConfig{BaseURL: server.URL + "/v1/", APIKey: "fixture", Model: "gpt-6.2-sol", HTTPClient: server.Client()}, "high")
	bound, err := chat.WithTools([]*schema.ToolInfo{{Name: "read_sample"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	messages := []*schema.Message{schema.SystemMessage("核查"), schema.UserMessage("查阅样本")}
	reply, err := bound.Generate(ctx, messages)
	if err != nil || len(reply.ToolCalls) != 1 || reply.ToolCalls[0].ID != "call_a" {
		t.Fatalf("工具转换失败: %+v %v", reply, err)
	}
	final, err := bound.Generate(ctx, append(messages, reply, schema.ToolMessage("已核查", "call_a")))
	if err != nil || final.Content != "完成" || calls != 2 {
		t.Fatalf("续接失败: %+v %v", final, err)
	}
}

const responseToolFixture = `{"status":"completed","output":[{"type":"reasoning","id":"rs_a","encrypted_content":"opaque","summary":[]},{"type":"function_call","id":"fc_a","call_id":"call_a","name":"read_sample","arguments":"{}","status":"completed"}]}`
const responseTextFixture = `{"status":"completed","output":[{"type":"message","id":"msg_a","role":"assistant","status":"completed","content":[{"type":"output_text","text":"完成","annotations":[]}]}]}`
