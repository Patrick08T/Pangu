package workflow

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func loopDefinition() Definition {
	d := Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Input: "修复 bug"}
	ids := []string{"discover", "reproduce", "fix", "selftest", "review", "qa"}
	for i, id := range ids {
		n := Node{ID: id, Executor: "codex", Instruction: id}
		if i > 0 {
			n.DependsOn = []string{ids[i-1]}
		}
		if i >= 3 {
			n.ReadOnly, n.OutputSchema = true, LoopGateSchema()
		}
		d.Nodes = append(d.Nodes, n)
	}
	d.Loops = []Loop{{ID: "bugfix", Nodes: ids, Gates: ids[3:]}}
	return d
}

func loopResult(in NodeInput, pass bool) NodeOutput {
	text := "完成"
	if in.Iteration.Gate {
		text = fmt.Sprintf(`{"passed":%t,"summary":"检查结论"}`, pass)
	}
	return NodeOutput{Text: text, ContentRevision: fmt.Sprint("tree-", in.Iteration.Round)}
}

func TestLoopRestartsOnEachGateFailureUntilAllPass(t *testing.T) {
	d := loopDefinition()
	var calls []string
	outputs, err := Run(d, func(inputs []NodeInput) ([]NodeOutput, error) {
		in := inputs[0]
		if in.Iteration == nil {
			t.Fatal("缺少循环轮次")
		}
		round, id := in.Iteration.Round, in.Node.ID
		if round > 4 {
			t.Fatal("全部通过后仍在循环")
		}
		calls = append(calls, fmt.Sprintf("%d/%s", round, id))
		if round > 1 && (in.Iteration.Feedback == nil || in.Iteration.Feedback.Round != round-1) {
			t.Fatal("失败证据未交接")
		}
		pass := !(round == 1 && id == "selftest" || round == 2 && id == "review" || round == 3 && id == "qa")
		return []NodeOutput{loopResult(in, pass)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 21 || outputs["qa"].Iteration.Round != 4 || outputs["discover"].Iteration.Round != 4 {
		t.Fatalf("没有完成四轮：%v", calls)
	}
}

func TestLoopResumesCurrentRoundWithoutRepeatingCompletedNodes(t *testing.T) {
	d := loopDefinition()
	reused := map[string]NodeOutput{}
	for _, id := range []string{"discover", "reproduce", "fix", "selftest"} {
		reused[id] = NodeOutput{Text: "完成", Iteration: &Iteration{ID: "bugfix", Round: 2}, ContentRevision: "tree-2"}
	}
	reused["selftest"] = NodeOutput{Text: `{"passed":true,"summary":"通过"}`, Iteration: &Iteration{ID: "bugfix", Round: 2, Gate: true}, ContentRevision: "tree-2"}
	var called []string
	_, err := RunFrom(d, reused, func(inputs []NodeInput) ([]NodeOutput, error) {
		in := inputs[0]
		called = append(called, in.Node.ID)
		if in.Iteration.Round != 2 {
			t.Fatal("续跑丢失轮次")
		}
		return []NodeOutput{loopResult(in, true)}, nil
	})
	if err != nil || !reflect.DeepEqual(called, []string{"review", "qa"}) {
		t.Fatalf("%v %v", called, err)
	}
}

func TestLoopExecutionErrorsDoNotBecomeBusinessIterations(t *testing.T) {
	want := errors.New("主动取消或预算耗尽")
	calls := 0
	_, err := Run(loopDefinition(), func([]NodeInput) ([]NodeOutput, error) { calls++; return nil, want })
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("%v %d", err, calls)
	}
}

func TestLoopRejectsMixedCodeRevisions(t *testing.T) {
	_, err := Run(loopDefinition(), func(inputs []NodeInput) ([]NodeOutput, error) {
		out := loopResult(inputs[0], true)
		if inputs[0].Node.ID == "review" {
			out.ContentRevision = "changed-tree"
		}
		return []NodeOutput{out}, nil
	})
	if err == nil {
		t.Fatal("混合代码版本被验收")
	}
}

func TestLoopRejectsInvalidBoundaries(t *testing.T) {
	cases := map[string]func(*Definition){
		"重叠":    func(d *Definition) { d.Loops = append(d.Loops, d.Loops[0]) },
		"可写验收":  func(d *Definition) { d.Nodes[4].ReadOnly = false },
		"未声明结构": func(d *Definition) { d.Nodes[4].OutputSchema = nil },
		"跳过验收":  func(d *Definition) { d.Nodes[4].When = &Condition{} },
		"提前外流": func(d *Definition) {
			d.Nodes = append(d.Nodes, Node{ID: "outside", Executor: "codex", Instruction: "消费", DependsOn: []string{"fix"}})
		},
		"跨块环": func(d *Definition) {
			d.Nodes = append(d.Nodes, Node{ID: "outside", Executor: "codex", Instruction: "消费", DependsOn: []string{"qa"}})
			d.Nodes[0].DependsOn = []string{"outside"}
		},
		"门后写入": func(d *Definition) { d.Loops[0].Gates = []string{"selftest", "qa"}; d.Nodes[4].ReadOnly = false },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			d := loopDefinition()
			change(&d)
			if _, err := d.Layers(); err == nil {
				t.Fatal("非法循环被接受")
			}
		})
	}
}

func TestLoopDefinitionChangeInvalidatesReusedPrefix(t *testing.T) {
	old := loopDefinition()
	next := loopDefinition()
	next.Loops = nil
	ids, err := Prefix(next, old, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatal("循环规则改变仍复用旧轮次")
	}
}

func TestLoopCompletionHasExplicitAcceptedOutcome(t *testing.T) {
	d := loopDefinition()
	outputs, err := Run(d, func(inputs []NodeInput) ([]NodeOutput, error) { return []NodeOutput{loopResult(inputs[0], true)}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got := BusinessOutcome(d, outputs, "succeeded"); got.State != "accepted" {
		t.Fatalf("完成后未评估验收: %+v", got)
	}
}

func TestLoopKeepsOuterNodesOnceAndExecutesParallelRepairLayer(t *testing.T) {
	d := loopDefinition()
	d.Nodes[0].DependsOn = []string{"source"}
	d.Nodes = append(d.Nodes, Node{ID: "source", Executor: "codex", Instruction: "准备"}, Node{ID: "end", Executor: "codex", Instruction: "汇总", DependsOn: []string{"qa"}}, Node{ID: "peer", Executor: "codex", Instruction: "独立诊断", DependsOn: []string{"discover"}})
	d.Nodes[2].DependsOn = append(d.Nodes[2].DependsOn, "peer")
	d.Loops[0].Nodes = append(d.Loops[0].Nodes, "peer")
	counts := map[string]int{}
	parallel := false
	_, err := Run(d, func(inputs []NodeInput) ([]NodeOutput, error) {
		parallel = parallel || len(inputs) == 2
		out := []NodeOutput{}
		for _, in := range inputs {
			counts[in.Node.ID]++
			if in.Iteration == nil {
				out = append(out, NodeOutput{Text: "外部完成"})
				continue
			}
			out = append(out, loopResult(in, !(in.Iteration.Round == 1 && in.Node.ID == "review")))
		}
		return out, nil
	})
	if err != nil || !parallel || counts["source"] != 1 || counts["end"] != 1 || counts["fix"] != 2 {
		t.Fatalf("错误调度: %v %v", counts, err)
	}
}

func TestLoopSkipsWholeBlockWhenExternalDependencyWasSkipped(t *testing.T) {
	for _, member := range []int{0, 5} {
		d := loopDefinition()
		d.Nodes[member].DependsOn = append(d.Nodes[member].DependsOn, "approved")
		d.Nodes = append(d.Nodes,
			Node{ID: "approved", Executor: "codex", Instruction: "审批入口"},
			Node{ID: "delivery", Executor: "codex", Instruction: "交付", DependsOn: []string{"qa"}})
		called := false
		outputs, err := RunFrom(d, map[string]NodeOutput{"approved": {Skipped: true}}, func([]NodeInput) ([]NodeOutput, error) {
			called = true
			return nil, errors.New("不应执行未获准的修改或交付")
		})
		if err != nil || called {
			t.Fatalf("外部条件未命中应跳过整个循环：%v", err)
		}
		for _, node := range d.Nodes {
			if !outputs[node.ID].Skipped || outputs[node.ID].Iteration != nil {
				t.Fatalf("%s 被误记为已执行循环", node.ID)
			}
		}
		if BusinessOutcome(d, outputs, "succeeded").State == "accepted" {
			t.Fatal("跳过循环不能冒充验收通过")
		}
	}
}
