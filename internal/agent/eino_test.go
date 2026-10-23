package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestExecuteEinoAgent(t *testing.T) {
	var received struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong model request")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"总结结果"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	input := workflow.NodeInput{
		Node:  workflow.Node{ID: "summary", Model: "test-model", Instruction: `输出 JSON {"answer": "..."}`},
		Input: "用户主题", Parents: map[string]workflow.NodeOutput{"research": {Text: "上游结果"}},
	}
	result, err := Execute(context.Background(), input)
	if err != nil || result.Text != "总结结果" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if received.Model != "test-model" || len(received.Messages) != 2 {
		t.Fatalf("request=%+v", received)
	}
	if received.Messages[0].Content != input.Node.Instruction {
		t.Fatalf("instruction changed: %+v", received.Messages)
	}
	if !strings.Contains(received.Messages[1].Content, "用户主题") || !strings.Contains(received.Messages[1].Content, "上游结果") {
		t.Fatal("missing input or parent context")
	}
}

func TestExecutePropagatesModelFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"model unavailable","type":"server_error"}}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	_, err := Execute(context.Background(), workflow.NodeInput{Node: workflow.Node{ID: "test", Model: "test", Instruction: "test"}})
	if err == nil {
		t.Fatal("model failure was swallowed")
	}
}

func TestExecuteRequiresCurrentCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := Execute(context.Background(), workflow.NodeInput{Node: workflow.Node{ID: "test", Model: "test", Instruction: "test"}})
	if err == nil {
		t.Fatal("accepted missing credentials")
	}
}
