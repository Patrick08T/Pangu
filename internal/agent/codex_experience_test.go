package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestCodexReceivesExperienceBodyAndProvenance(t *testing.T) {
	setupCodex(t)
	input := codexInput("test-model")
	input.Experiences = []workflow.Experience{{
		Reference: workflow.MemoryReference{VersionID: "version-a", BodyHash: "digest", DeliveryRevision: 7},
		Kind:      "heuristic", Conditions: "timeout only", Content: json.RawMessage(`{"practice":"inspect timeout evidence"}`),
		Sources: []workflow.MemorySource{{ID: "source-a", Kind: "command", EvidenceHash: "source-digest"}},
	}}
	out, err := Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var captured struct {
		Input workflow.NodeInput `json:"input"`
	}
	if err := json.Unmarshal([]byte(out.Text), &captured); err != nil {
		t.Fatal(err)
	}
	if len(captured.Input.Experiences) != 1 {
		t.Fatalf("Codex lost memory data: %s", out.Text)
	}
	experience := captured.Input.Experiences[0]
	if string(experience.Content) != `{"practice":"inspect timeout evidence"}` || experience.Reference.DeliveryRevision != 7 || len(experience.Sources) != 1 || experience.Sources[0].EvidenceHash != "source-digest" {
		t.Fatalf("Codex memory payload lost frozen evidence: %+v", experience)
	}
	if strings.Contains(captured.Input.Node.Instruction, "inspect timeout evidence") {
		t.Fatal("task memory became trusted role instruction")
	}
}
