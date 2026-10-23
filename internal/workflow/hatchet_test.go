package workflow

import (
	"errors"
	"testing"
)

func TestRunSubmitsParallelNodesBeforeDependents(t *testing.T) {
	var calls int
	result, err := Run(definition(), func(inputs []NodeInput) ([]NodeOutput, error) {
		calls++
		if calls == 1 {
			if len(inputs) != 2 {
				t.Fatalf("parallel wave: %v", inputs)
			}
			return []NodeOutput{{Text: "L", RunID: "run-left"}, {Text: "R", RunID: "run-right"}}, nil
		}
		if len(inputs) != 1 || inputs[0].Parents["left"].Text != "L" || inputs[0].Parents["right"].Text != "R" {
			t.Fatalf("dependency outputs: %+v", inputs)
		}
		return []NodeOutput{{Text: "summary"}}, nil
	})
	if err != nil || calls != 2 || result["summary"].Text != "summary" {
		t.Fatalf("result=%v calls=%d err=%v", result, calls, err)
	}
}

func TestRunStopsDownstreamOnFailure(t *testing.T) {
	want := errors.New("节点重试耗尽")
	calls := 0
	_, err := Run(definition(), func([]NodeInput) ([]NodeOutput, error) { calls++; return nil, want })
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRunRejectsInvalidInputBeforeScheduling(t *testing.T) {
	_, err := Run(Definition{}, func([]NodeInput) ([]NodeOutput, error) { t.Fatal("scheduled invalid input"); return nil, nil })
	if err == nil {
		t.Fatal("missing validation error")
	}
}

func TestRunRejectsIncompleteBatch(t *testing.T) {
	_, err := Run(definition(), func([]NodeInput) ([]NodeOutput, error) { return nil, nil })
	if err == nil {
		t.Fatal("accepted incomplete outputs")
	}
}
