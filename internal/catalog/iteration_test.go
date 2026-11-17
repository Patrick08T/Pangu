package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func backendDefinition(t *testing.T) workflow.Definition {
	t.Helper()
	raw, err := os.ReadFile("../../examples/workflows/backend-delivery.json")
	if err != nil {
		t.Fatal(err)
	}
	var item NewObject
	if err = json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if err = item.Validate(); err != nil {
		t.Fatal(err)
	}
	d := workflow.Definition{ProjectID: "00000000-0000-0000-0000-000000000001", Input: "合成需求", Loops: item.Spec.Flow.Loops}
	for _, b := range item.Spec.Flow.Nodes {
		instruction := b.Task
		if instruction == "" {
			instruction = "复用角色的阶段任务"
		}
		d.Nodes = append(d.Nodes, workflow.Node{ID: b.ID, Executor: "codex", Instruction: instruction, Handoff: testHandoff(),
			ReadOnly: b.ReadOnly || b.ID == "plan" || b.ID == "qa_design", DependsOn: b.DependsOn,
			AgentVersion: b.AgentVersion, Approval: b.Approval, OutputSchema: b.OutputSchema, ReportSchema: b.ReportSchema})
	}
	return d
}

func TestIterationBlueprintsRepeatOnlyRepairBeforeDelivery(t *testing.T) {
	for name, d := range map[string]workflow.Definition{"市场迭代": marketDefinition(t), "后端交付": backendDefinition(t)} {
		t.Run(name, func(t *testing.T) {
			if len(d.Loops) != 1 {
				t.Fatal("缺少修复循环")
			}
			counts, approvals := map[string]int{}, 0
			outputs, err := workflow.RunControlled(d, nil, func(in *workflow.NodeInput, _ map[string]workflow.NodeOutput) error {
				if in.Node.Approval != nil {
					approvals++
				}
				return nil
			}, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
				results := []workflow.NodeOutput{}
				for _, in := range inputs {
					counts[in.Node.ID]++
					if in.Iteration != nil && in.Iteration.Round > 2 {
						t.Fatal("质量门通过后没有退出")
					}
					if in.Iteration != nil && in.Iteration.Round == 2 && in.Iteration.Feedback == nil {
						t.Fatal("下一轮缺少失败反馈")
					}
					results = append(results, iterationOutput(in, in.Iteration == nil || in.Iteration.Round == 2))
				}
				return results, nil
			})
			if err != nil || approvals != 1 {
				t.Fatalf("审批或循环执行错误：%d %v", approvals, err)
			}
			for _, n := range d.Nodes {
				want := 1
				if d.LoopFor(n.ID) != nil {
					want = 2
					if outputs[n.ID].Iteration.Round != 2 {
						t.Fatal("复用了旧轮成果")
					}
				}
				if counts[n.ID] != want {
					t.Errorf("%s 执行次数=%d，期望=%d", n.ID, counts[n.ID], want)
				}
			}
		})
	}
}

func iterationOutput(in workflow.NodeInput, pass bool) workflow.NodeOutput {
	if in.Iteration != nil && in.Iteration.Gate {
		text := fmt.Sprintf(`{"passed":%t,"summary":"独立验证代码修复"}`, pass)
		out := workflow.NodeOutput{Text: text, ContentRevision: fmt.Sprint("tree-", in.Iteration.Round)}
		if in.Node.ReportSchema != nil {
			out.ReportIDs, out.ReportSHA256 = []string{"00000000-0000-0000-0000-000000000001"}, workflow.ReportHash(text)
		}
		return out
	}
	if in.Node.OutputSchema == nil {
		return workflow.NodeOutput{Text: "合成阶段结果"}
	}
	return marketOutput(in.Node, true, "合成路由结果")
}

func TestIterationBlueprintsKeepEvidenceWaitAndDeploymentOutsideLoop(t *testing.T) {
	for _, d := range []workflow.Definition{marketDefinition(t), backendDefinition(t)} {
		if len(d.Loops) != 1 {
			t.Fatal("缺少修复循环")
		}
		for _, n := range d.Nodes {
			if d.LoopFor(n.ID) != nil && strings.Contains(n.Instruction, "此处为一次明确修复阶段") {
				t.Fatalf("%s 仍保留与平台循环冲突的单次修复指令", n.ID)
			}
			if n.Approval != nil || n.ID == "local_deploy" || n.ID == "qa_test" || n.ReportSchema != nil && n.ReportSchema.Properties["evidence_blockers"].Type != "" {
				if d.LoopFor(n.ID) != nil {
					t.Fatalf("%s 不应在自动修复循环内重复执行", n.ID)
				}
			}
			if d.LoopFor(n.ID) != nil && n.ReadOnly && !strings.Contains(n.Instruction, "ocr delegate preview") {
				t.Fatalf("%s 缺少累计补丁 OCR 审查", n.ID)
			}
		}
	}
}
