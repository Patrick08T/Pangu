package workflow

import (
	"encoding/json"
	"testing"
)

func reviewReport(t *testing.T) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"tasks": []map[string]string{{"id": "DEV-1"}}, "evidence": []string{"来源依据"}, "review": Review{
		Title: "方向与任务", Summary: "确认本轮范围", Items: []ReviewItem{{ID: "scope", Category: "范围", Title: "文档交付", Proposal: "仅交付文档", Rationale: "先验证需求", EvidenceRefs: []string{"#/evidence/0"}, TaskIDs: []string{"DEV-1"}}}, Resolutions: []ReviewResolution{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestReviewRejectsIncompleteAndUnboundItems(t *testing.T) {
	good := reviewReport(t)
	if _, err := ParseReview(good); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"duplicate", "missing_title", "unknown_task", "unknown_evidence", "unknown_field"} {
		t.Run(kind, func(t *testing.T) {
			var report map[string]any
			_ = json.Unmarshal([]byte(good), &report)
			review := report["review"].(map[string]any)
			items := review["items"].([]any)
			item := items[0].(map[string]any)
			switch kind {
			case "duplicate":
				review["items"] = append(items, item)
			case "missing_title":
				delete(item, "title")
			case "unknown_task":
				item["task_ids"] = []string{"missing"}
			case "unknown_evidence":
				item["evidence_refs"] = []string{"#/evidence/9"}
			case "unknown_field":
				item["approved"] = true
			}
			body, _ := json.Marshal(report)
			if _, err := ParseReview(string(body)); err == nil {
				t.Fatal("接受无效审核项")
			}
		})
	}
}

func TestApprovalRequiresReadOnlyAncestorRevisionPlan(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-4000-8000-000000000001", Nodes: []Node{
		{ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划"},
		{ID: "build", Executor: "codex", Instruction: "实施", DependsOn: []string{"plan"}, Approval: &Approval{SourceNode: "plan", RevisionNodes: []string{"plan"}}},
	}}
	if _, err := d.Layers(); err != nil {
		t.Fatal(err)
	}
	d.Nodes[0].ReadOnly = false
	if _, err := d.Layers(); err == nil {
		t.Fatal("允许写入角色在批准前修订")
	}
	d.Nodes[0].ReadOnly = true
	d.Nodes[1].Approval.SourceNode = "missing"
	if _, err := d.Layers(); err == nil {
		t.Fatal("接受不存在的报告来源")
	}
}

func TestControlledRunPropagatesRevisedParents(t *testing.T) {
	d := Definition{Nodes: []Node{{ID: "plan", Executor: "eino", Model: "test", Instruction: "规划"}, {ID: "build", Executor: "eino", Model: "test", Instruction: "实施", DependsOn: []string{"plan"}}}}
	before := func(input *NodeInput, outputs map[string]NodeOutput) error {
		if input.Node.ID == "build" {
			outputs["plan"] = NodeOutput{Text: "新版"}
			*input = d.NodeInput(input.Node, outputs)
		}
		return nil
	}
	out, err := RunControlled(d, nil, before, func(inputs []NodeInput) ([]NodeOutput, error) {
		if inputs[0].Node.ID == "build" && inputs[0].Parents["plan"].Text != "新版" {
			t.Fatal("实施收到旧计划")
		}
		return []NodeOutput{{Text: "完成"}}, nil
	})
	if err != nil || out["plan"].Text != "新版" {
		t.Fatalf("新版没有保留: %+v %v", out, err)
	}
}

func TestApprovalRejectsStaleSiblingAndParallelInputs(t *testing.T) {
	d := Definition{ProjectID: "00000000-0000-4000-8000-000000000001", Nodes: []Node{
		{ID: "plan", Executor: "codex", ReadOnly: true, Instruction: "规划"},
		{ID: "summary", Executor: "eino", Model: "test", Instruction: "摘要", DependsOn: []string{"plan"}},
		{ID: "source", Executor: "codex", ReadOnly: true, Instruction: "汇总", DependsOn: []string{"plan"}},
		{ID: "build", Executor: "eino", Model: "test", Instruction: "实施", DependsOn: []string{"source"}, Approval: &Approval{SourceNode: "source", RevisionNodes: []string{"plan", "source"}}},
	}}
	if _, err := d.Layers(); err == nil {
		t.Fatal("允许已完成的旁支继续持有修订前计划")
	}
	d.Nodes[1].DependsOn = []string{"source"}
	if _, err := d.Layers(); err == nil {
		t.Fatal("允许同层节点在审批前构造旧输入")
	}
	d.Nodes[1].DependsOn = []string{"build"}
	if _, err := d.Layers(); err != nil {
		t.Fatal(err)
	}
}
