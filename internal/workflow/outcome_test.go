package workflow

import "testing"

func TestBusinessOutcomeRequiresExplicitTerminalAcceptance(t *testing.T) {
	d := Definition{Nodes: []Node{{ID: "plan"}, {ID: "accept", DependsOn: []string{"plan"}}}}
	for _, tc := range []struct{ name, text, execution, state string }{
		{"部分验收不能随执行成功变成通过", `{"iteration_complete":false,"ready":false,"status":"partial","summary":"缺证据"}`, "succeeded", "needs_attention"},
		{"显式完成", `{"iteration_complete":true,"ready":true,"status":"complete"}`, "succeeded", "accepted"},
		{"矛盾结论不能通过", `{"iteration_complete":true,"ready":false,"status":"partial"}`, "succeeded", "needs_attention"},
		{"缺少验收不是通过", `{"summary":"完成"}`, "succeeded", "not_assessed"},
		{"引擎失败不能通过", `{"iteration_complete":true,"ready":true,"status":"complete"}`, "failed", "needs_attention"},
		{"运行尚未结束", `{"iteration_complete":true,"ready":true,"status":"complete"}`, "running", "pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := BusinessOutcome(d, map[string]NodeOutput{"accept": {Text: tc.text}}, tc.execution)
			if out.State != tc.state {
				t.Fatalf("%+v", out)
			}
		})
	}
}

func TestBusinessOutcomeDoesNotAcceptSkippedOrIncompleteLeaves(t *testing.T) {
	d := Definition{Nodes: []Node{{ID: "first"}, {ID: "second"}}}
	outputs := map[string]NodeOutput{"first": {Text: `{"iteration_complete":true,"ready":true,"status":"complete"}`}, "second": {Skipped: true}}
	if out := BusinessOutcome(d, outputs, "succeeded"); out.State == "accepted" {
		t.Fatal(out)
	}
	outputs["second"] = NodeOutput{Text: `{"iteration_complete":false,"ready":false,"status":"partial"}`}
	if out := BusinessOutcome(d, outputs, "succeeded"); out.State != "needs_attention" {
		t.Fatal(out)
	}
}
