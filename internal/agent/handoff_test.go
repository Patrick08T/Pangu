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

func TestHandoffReachesEinoAndCodexWithoutMutatingSnapshot(t *testing.T) {
	h := &workflow.HandoffContract{Objective: "交接目标甲", Inputs: "上游材料乙", Deliverables: "交付丙", Acceptance: "验收丁", FailureHandling: "失败戊"}
	for _, engine := range []string{"eino", "codex"} {
		t.Run(engine, func(t *testing.T) {
			var instruction string
			input := codexInput("model")
			if engine == "codex" {
				input.Workspace = setupCodex(t)
			} else {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body struct {
						Messages []struct {
							Content string `json:"content"`
						} `json:"messages"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					instruction = body.Messages[0].Content
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"完成"},"finish_reason":"stop"}]}`))
				}))
				defer server.Close()
				t.Setenv("OPENAI_BASE_URL", server.URL)
				t.Setenv("OPENAI_API_KEY", "local-test")
			}
			input.Node.Executor, input.Node.Handoff = engine, h
			original := input.Node.Instruction
			out, err := Execute(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if engine == "codex" {
				var body struct {
					Input workflow.NodeInput `json:"input"`
				}
				if err := json.Unmarshal([]byte(out.Text), &body); err != nil {
					t.Fatal(err)
				}
				instruction = body.Input.Node.Instruction
			}
			for _, text := range []string{original, h.Objective, h.Inputs, h.Deliverables, h.Acceptance, h.FailureHandling} {
				if !strings.Contains(instruction, text) {
					t.Fatalf("%s 执行未收到契约字段 %s", engine, text)
				}
			}
			if input.Node.Instruction != original {
				t.Fatal("契约注入改写了原始快照")
			}
		})
	}
}
