package workflow

import (
	"encoding/json"
	"testing"
)

func TestNodeInputPreservesApprovalGeneration(t *testing.T) {
	var input NodeInput
	if err := json.Unmarshal([]byte(`{"approval_generation":2,"node":{"id":"implement"}}`), &input); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err = json.Unmarshal(payload, &actual); err != nil {
		t.Fatal(err)
	}
	if actual["approval_generation"] != float64(2) {
		t.Fatalf("replayed input lost approval generation: %s", payload)
	}
}

func TestRunBeforeHookSeesBoundParameters(t *testing.T) {
	d := Definition{Input: `{"component":"payments"}`, InputSchema: &Shape{Type: "object", Required: []string{"component"},
		Properties: map[string]Shape{"component": {Type: "string"}}}, Nodes: []Node{{
		ID: "diagnose", Executor: "eino", Model: "test", Instruction: "查明失败原因",
		Bindings: map[string]FieldBinding{"module": {From: "$input", Pointer: "/component", Type: "string"}},
	}}}
	prepared := false
	before := func(input *NodeInput, _ map[string]NodeOutput) error {
		if input.Parameters["module"] != "payments" {
			t.Fatalf("preparation cannot select matching experience before binding: %#v", input.Parameters)
		}
		prepared = true
		return nil
	}
	_, err := RunControlled(d, nil, before, func(inputs []NodeInput) ([]NodeOutput, error) {
		if !prepared || len(inputs) != 1 || inputs[0].Parameters["module"] != "payments" {
			t.Fatal("execution did not receive prepared bound input")
		}
		return []NodeOutput{{Text: "ok"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
