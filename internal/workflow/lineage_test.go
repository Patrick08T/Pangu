package workflow

import (
	"encoding/json"
	"testing"
)

func TestEffectiveInputHashBindsLearningAndUpstreamContext(t *testing.T) {
	base := NodeInput{Node: Node{ID: "fix", Instruction: "verify first"}, Input: "task", Parameters: map[string]any{"module": "payments"}, Parents: map[string]NodeOutput{"plan": {Text: "read only"}}}
	original, err := EffectiveInputHash(base)
	if err != nil || len(original) != 64 {
		t.Fatalf("missing effective-input fingerprint: %q %v", original, err)
	}
	changes := []func(*NodeInput){
		func(in *NodeInput) { in.ApprovalGeneration = 2 },
		func(in *NodeInput) { in.Node.Instruction = "different policy" },
		func(in *NodeInput) { in.Parameters["module"] = "login" },
		func(in *NodeInput) { in.Parents["plan"] = NodeOutput{Text: "write code"} },
		func(in *NodeInput) {
			in.Experiences = []Experience{{Reference: MemoryReference{VersionID: "v1", BodyHash: "abc", DeliveryRevision: 2}, Content: json.RawMessage(`{"practice":"check"}`)}}
		},
	}
	for i, change := range changes {
		data, _ := json.Marshal(base)
		var other NodeInput
		if err := json.Unmarshal(data, &other); err != nil {
			t.Fatal(err)
		}
		change(&other)
		hash, err := EffectiveInputHash(other)
		if err != nil || hash == original {
			t.Fatalf("changed logical input %d reused fingerprint: %v", i, err)
		}
	}
	base.RunID, base.NodeRunID, base.Workspace, base.ProcessScope = "run", "attempt", "temporary", "process"
	base.ApprovalID, base.ApprovalGeneration = "gate", 1
	hash, err := EffectiveInputHash(base)
	if err != nil || hash != original {
		t.Fatalf("physical retry changed logical input identity: %v", err)
	}
}

func TestOutputFingerprintCoversReusePayload(t *testing.T) {
	base := NodeOutput{Text: "approved patch", RunID: "node-run", ReportIDs: []string{"report-1"}, ContentRevision: "revision-a"}
	original, err := OutputFingerprint(base)
	if err != nil || len(original) != 64 {
		t.Fatalf("missing output fingerprint: %q %v", original, err)
	}
	for _, change := range []func(*NodeOutput){
		func(out *NodeOutput) { out.Text = "tampered patch" },
		func(out *NodeOutput) { out.ContentRevision = "revision-b" },
		func(out *NodeOutput) { out.ReportIDs = []string{"report-2"} },
	} {
		other := base
		change(&other)
		hash, err := OutputFingerprint(other)
		if err != nil || hash == original {
			t.Fatalf("mutated reusable output retained fingerprint: %v", err)
		}
	}
}
