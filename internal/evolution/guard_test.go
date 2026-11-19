package evolution

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func controlledDefinition() workflow.Definition {
	return workflow.Definition{ProjectID: "11111111-1111-4111-8111-111111111111", Nodes: []workflow.Node{{ID: "answer", Executor: "eino", Instruction: "返回JSON答案", ConnectionVersion: "22222222-2222-4222-8222-222222222222"}}}
}

func TestEnvironmentRejectsUncontrolledExecution(t *testing.T) {
	d := controlledDefinition()
	env := Environment{Backend: "eino_json_v1", Enabled: true, ConnectionVersion: d.Nodes[0].ConnectionVersion}
	if err := ValidateExecution(env, d); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Environment, *workflow.Definition){
		"disabled":                func(e *Environment, d *workflow.Definition) { e.Enabled = false },
		"codex_backend":           func(e *Environment, d *workflow.Definition) { e.Backend = "codex" },
		"codex_node":              func(e *Environment, d *workflow.Definition) { d.Nodes[0].Executor = "codex" },
		"unregistered_connection": func(e *Environment, d *workflow.Definition) { e.ConnectionVersion = "other" },
		"implicit_connection": func(e *Environment, d *workflow.Definition) {
			e.ConnectionVersion = ""
			d.Nodes[0].ConnectionVersion = ""
		},
		"approval": func(e *Environment, d *workflow.Definition) { d.Nodes[0].Approval = &workflow.Approval{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := controlledDefinition()
			changed := env
			mutate(&changed, &candidate)
			if !errors.Is(ValidateExecution(changed, candidate), ErrIsolation) {
				t.Fatal("未拒绝非受控执行")
			}
		})
	}
}

func TestJSONCheckerNeverAcceptsSelfReportedPassOrUnknownCriteria(t *testing.T) {
	cases := []struct {
		output string
		want   bool
	}{
		{`{"answer":42}`, true}, {`{"answer":41}`, false},
		{`{"passed":true}`, false}, {`{"answer":41,"answer":42}`, false},
		{"答案是42", false}, {`{"answer":42} {"answer":41}`, false},
	}
	for _, tc := range cases {
		ok, err := checkJSONResult(`{"answer":42}`, []string{"json_equal"}, tc.output)
		if err != nil || ok != tc.want {
			t.Fatalf("%s: passed=%v err=%v", tc.output, ok, err)
		}
	}
	if _, err := checkJSONResult(`{}`, []string{"人工确认成功"}, `{}`); !errors.Is(err, ErrChecker) {
		t.Fatal("未知检查器被接受")
	}
	if _, err := checkJSONResult(`broken`, []string{"json_equal"}, `{}`); !errors.Is(err, ErrChecker) {
		t.Fatal("无效Expected被接受")
	}
}
