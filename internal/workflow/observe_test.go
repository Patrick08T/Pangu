package workflow

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestOutputObserverRecordsExecutedAndSkippedNodes(t *testing.T) {
	d := Definition{Input: `{"enabled":true}`, InputSchema: &Shape{Type: "object", Properties: map[string]Shape{"enabled": {Type: "boolean"}}, Required: []string{"enabled"}}, Nodes: []Node{
		{ID: "yes", Executor: "eino", Model: "test", Instruction: "execute", When: &Condition{Field: FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`true`)}},
		{ID: "no", Executor: "eino", Model: "test", Instruction: "skip", When: &Condition{Field: FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}, Equals: json.RawMessage(`false`)}},
		{ID: "child", Executor: "eino", Model: "test", Instruction: "skip dependent", DependsOn: []string{"no"}},
	}}
	seen := map[string]int{}
	observe := func(input NodeInput, out *NodeOutput) error {
		seen[input.Node.ID]++
		if input.Node.ID == "child" && input.Parents["no"].Lineage == nil {
			t.Fatal("skip dependency lost upstream proof before observation")
		}
		out.Lineage = &OutputLineage{Version: 1, ArtifactID: input.Node.ID}
		return nil
	}
	out, err := RunAudited(d, nil, nil, observe, func(inputs []NodeInput) ([]NodeOutput, error) {
		if len(inputs) != 1 || inputs[0].Node.ID != "yes" {
			t.Fatalf("executed skipped nodes: %+v", inputs)
		}
		return []NodeOutput{{Text: "done"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"yes", "no", "child"} {
		if seen[id] != 1 || out[id].Lineage == nil {
			t.Fatalf("output %s bypassed proof observer: %+v %+v", id, seen, out)
		}
	}
}

func TestOutputObserverFailureBlocksDownstreamExecution(t *testing.T) {
	d := Definition{Nodes: []Node{{ID: "first", Executor: "eino", Model: "test", Instruction: "one"}, {ID: "second", Executor: "eino", Model: "test", Instruction: "two", DependsOn: []string{"first"}}}}
	calls := 0
	refused := errors.New("artifact persistence unavailable")
	_, err := RunAudited(d, nil, nil, func(NodeInput, *NodeOutput) error { return refused }, func(inputs []NodeInput) ([]NodeOutput, error) { calls++; return []NodeOutput{{Text: "done"}}, nil })
	if !errors.Is(err, refused) || calls != 1 {
		t.Fatalf("unproven result continued: calls=%d err=%v", calls, err)
	}
}
