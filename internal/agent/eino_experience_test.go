package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestEinoReceivesExperienceAsStructuredTaskData(t *testing.T) {
	var messages []struct{ Role, Content string }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		messages = request.Messages
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")
	var input workflow.NodeInput
	raw := []byte(`{"node":{"id":"check","executor":"eino","model":"test","instruction":"Verify evidence"},"input":"timeout","experiences":[{"reference":{"version_id":"version-a","body_hash":"body-digest","delivery_revision":7},"kind":"heuristic","conditions":"timeout only","content":{"practice":"inspect the verified timeout source"},"sources":[{"id":"source-a","kind":"command","evidence_hash":"source-digest"}]}]}`)
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[0].Content != "Verify evidence" {
		t.Fatalf("experience changed the trusted role instruction: %+v", messages)
	}
	var payload struct {
		Experiences []struct {
			Content map[string]string   `json:"content"`
			Sources []map[string]string `json:"sources"`
		} `json:"experiences"`
	}
	if err := json.Unmarshal([]byte(messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Experiences) != 1 || payload.Experiences[0].Content["practice"] != "inspect the verified timeout source" || len(payload.Experiences[0].Sources) != 1 {
		t.Fatalf("model did not receive evidence-backed experience body: %s", messages[1].Content)
	}
}
