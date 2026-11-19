package evolution

import (
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestResearchReplayEvaluatesOnlyAuthorizedCollector(t *testing.T) {
	d := controlledDefinition()
	d.Nodes[0].Role, d.Nodes[0].Mode = "product_manager", "research"
	d.Nodes[0].AgentVersion = "33333333-3333-4333-8333-333333333333"
	d.Nodes[0].Policy = &workflow.Policy{Retries: 0}
	d.Nodes = append(d.Nodes, workflow.Node{ID: "delivery", Executor: "eino", Instruction: "delivery", ConnectionVersion: d.Nodes[0].ConnectionVersion, DependsOn: []string{"answer"}, AgentVersion: "44444444-4444-4444-8444-444444444444"})
	env := Environment{Backend: "eino_research_replay_v1", Enabled: true, ConnectionVersion: d.Nodes[0].ConnectionVersion}
	if err := ValidateExecution(env, d); err != nil {
		t.Fatal(err)
	}
	if err := validateTargets(env, d, CandidateRequest{BaselineVersion: d.Nodes[0].AgentVersion}); err != nil {
		t.Fatal(err)
	}
	if err := validateTargets(env, d, CandidateRequest{BaselineVersion: d.Nodes[1].AgentVersion}); !errors.Is(err, ErrIsolation) {
		t.Fatal("下游交付角色不可借采集证据晋级")
	}
	env.Backend = "eino_json_v1"
	if err := ValidateExecution(env, d); !errors.Is(err, ErrIsolation) {
		t.Fatal("无工具环境不可执行调研节点")
	}
	env.Backend = "eino_research_replay_v1"
	d.Nodes[0].When = &workflow.Condition{}
	if err := ValidateExecution(env, d); !errors.Is(err, ErrIsolation) {
		t.Fatal("不能绕过采集节点条件")
	}
}

func TestReplayCheckerRejectsWrongDateAndExtraFacts(t *testing.T) {
	const expected = `{"facts":[{"statement":"收入增长 12%","source_id":"d0","quote":"收入增长 12%","source_date":"2026-01-01"}]}`
	const correct = `{"mode":"research","claims":[{"kind":"fact","statement":"收入增长 12%","evidence":[{"source_id":"d0","quote":"报告：收入增长 12%。","source_date":"2026-01-01"}]}]}`
	for _, tc := range []struct {
		text string
		pass bool
	}{
		{correct, true},
		{`{"mode":"research","claims":[{"kind":"fact","statement":"收入增长 12%","evidence":[{"source_id":"d0","quote":"收入增长 12%","source_date":"2026-01-02"}]}]}`, false},
		{`{"mode":"research","claims":[{"kind":"fact","statement":"收入增长 12%","evidence":[{"source_id":"d0","quote":"收入增长 12%","source_date":"2026-01-01"}]},{"kind":"fact","statement":"净收入增长 12%"}]}`, false},
		{`{"passed":true}`, false},
	} {
		pass, _, err := checkReplayResult(expected, []string{"research_replay_v1"}, tc.text)
		if err != nil || pass != tc.pass {
			t.Fatalf("pass=%t err=%v: %s", pass, err, tc.text)
		}
	}
}
