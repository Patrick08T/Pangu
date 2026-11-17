package catalog

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func TestMarketIterationPublishedRolesCompile(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	connection, err := Create(ctx, ns, NewObject{Kind: "connection", Name: "研究连接", Spec: Spec{Connection: &Connection{Provider: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	cv, err := Publish(ctx, ns, connection.ID, connection.Revision)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"91e28bd0-cba8-4a54-a94a-af46ac6f9570": "market-researcher", "9076db0d-4e31-48dd-92b3-910403db9dc0": "pm-planner",
		"5ccfe0f4-10a9-4fc4-8f38-e03d8a733ff1": "dev-engineer", "25fa8397-390b-4123-9e06-93473171d8aa": "qa-engineer",
		"ddae0c7f-3e3c-4bac-9416-b85a17a18e2d": "game-performance-engineer", "9b777c68-42eb-4547-9450-d16e14ac4722": "game-security-engineer",
		"6b5b4211-11a9-4e08-8d77-d8bd0927c2d2": "game-release-engineer", "e7269faf-1968-4789-9595-354bdb6997ac": "acceptance-reviewer"}
	versions := map[string]string{}
	for original, name := range files {
		var role NewObject
		raw, readErr := os.ReadFile("../../examples/templates/" + name + ".json")
		if readErr != nil || json.Unmarshal(raw, &role) != nil {
			t.Fatal("无法读取角色", name, readErr)
		}
		if role.Spec.Agent.Executor == "eino" {
			role.Spec.Agent.ConnectionVersion = cv.ID
		}
		item, createErr := Create(ctx, ns, role)
		if createErr != nil {
			t.Fatal(name, createErr)
		}
		v, publishErr := Publish(ctx, ns, item.ID, item.Revision)
		if publishErr != nil {
			t.Fatal(name, publishErr)
		}
		versions[original] = v.ID
	}
	var item NewObject
	raw, err := os.ReadFile("../../examples/workflows/market-driven-iteration.json")
	if err != nil || json.Unmarshal(raw, &item) != nil {
		t.Fatal("无法读取流程", err)
	}
	for i := range item.Spec.Flow.Nodes {
		item.Spec.Flow.Nodes[i].AgentVersion = versions[item.Spec.Flow.Nodes[i].AgentVersion]
	}
	obj, err := Create(ctx, ns, item)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, obj.ID, obj.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, v.ID, project, marketDefinition(t).Input)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(d)
	if len(data) > 256*1024 || len(d.Nodes) != 25 {
		t.Fatal("流程超过提交限制或缺少步骤", len(data))
	}
}

func marketDefinition(t *testing.T) workflow.Definition {
	t.Helper()
	raw, err := os.ReadFile("../../examples/workflows/market-driven-iteration.json")
	if err != nil {
		t.Fatal(err)
	}
	var item NewObject
	if err = json.Unmarshal(raw, &item); err != nil || item.Validate() != nil {
		t.Fatal("市场迭代流程定义无效", err)
	}
	d := workflow.Definition{ProjectID: uuid.NewString(), InputSchema: item.Spec.Flow.Parameters, Budget: item.Spec.Flow.Budget, Loops: item.Spec.Flow.Loops,
		Input: `{"question":"合成流程验证","context":"已有产品与路线图","constraints":"仅验证路由","public_queries":["公开产品说明"]}`}
	for _, binding := range item.Spec.Flow.Nodes {
		node := workflow.Node{ID: binding.ID, Executor: "codex", Instruction: binding.Task, Handoff: testHandoff(),
			DependsOn: binding.DependsOn, ReadOnly: binding.ReadOnly, OutputSchema: binding.OutputSchema, ReportSchema: binding.ReportSchema, When: binding.When, Approval: binding.Approval}
		if binding.ID == "market_research" {
			node.Executor, node.Role, node.Mode = "eino", "product_manager", "research"
			node.Model = "test"
		}
		d.Nodes = append(d.Nodes, node)
	}
	return d
}

func TestMarketIterationBlueprintReviewBeforeWrite(t *testing.T) {
	d := marketDefinition(t)
	layers, err := d.Layers()
	if err != nil || len(layers) >= len(d.Nodes) || len(d.Nodes) != 25 {
		t.Fatal("流程必须保留完整阶段并允许独立只读核验并行", err)
	}
	approvals, passedApproval := 0, false
	for _, n := range d.Nodes {
		if n.Approval != nil {
			approvals++
			passedApproval = true
			if !n.Approval.ExecutionPlan || n.ID != "apply_plan" || n.When == nil || n.When.Field.From != "execution_plan" || n.When.Field.Pointer != "/ready" {
				t.Fatal("审批没有绑定完整执行计划")
			}
		}
		if n.Executor == "codex" && !n.ReadOnly && !passedApproval {
			t.Fatalf("%s 在人工确认前可修改文件", n.ID)
		}
		if strings.Contains(n.Instruction, "工具上限") || strings.Contains(n.Instruction, "最多 10 次") {
			t.Fatalf("%s 恢复了旧工具限制", n.ID)
		}
	}
	if approvals != 1 || d.Nodes[len(d.Nodes)-1].ID != "acceptance" {
		t.Fatal("必须只有一次实施前确认并以独立验收结束")
	}
}

func TestMarketIterationExcludesHumanValidationWork(t *testing.T) {
	d := marketDefinition(t)
	for _, n := range d.Nodes {
		if !strings.Contains(n.Instruction, "【自动化范围】") {
			t.Errorf("%s 未明确规划及验收排除范围", n.ID)
		}
		for _, old := range []string{"样本招募方案", "需求强度、替代品不足、触达与付费证据", "市场采纳及商业收益单列后续验证"} {
			if strings.Contains(n.Instruction, old) {
				t.Errorf("%s 仍主动布置人力验证：%s", n.ID, old)
			}
		}
		if n.ID == "acceptance" && !strings.Contains(n.Instruction, "**范围说明：") {
			t.Error("最终结论缺少可选加粗范围说明")
		}
	}
}

func TestMarketPlanBindsExecutionGraphAndEvidenceWait(t *testing.T) {
	d := marketDefinition(t)
	for _, n := range d.Nodes {
		if n.ID == "execution_plan" && n.ReportSchema.Properties["execution_plan"].Type != "object" {
			t.Fatal("计划缺少可校验的执行图")
		}
		if n.ID == "qa_recheck" || n.ID == "performance_recheck" || n.ID == "acceptance" {
			if !n.ReadOnly || n.ReportSchema == nil || n.ReportSchema.Properties["evidence_blockers"].Type != "array" {
				t.Fatal("核验未提供持久补证契约", n.ID)
			}
		}
	}
}

func TestMarketIterationBlueprintBlockedAndRepairPaths(t *testing.T) {
	d := marketDefinition(t)
	for _, blocked := range []bool{false, true} {
		called := map[string]bool{}
		_, err := workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
			results := []workflow.NodeOutput{}
			for _, input := range inputs {
				n := input.Node
				called[n.ID] = true
				if n.ID == "market_research" {
					results = append(results, workflow.NodeOutput{Text: "合成研究报告"})
					continue
				}
				ready := !(blocked && n.ID == "execution_plan") && n.ID != "qa_validation"
				results = append(results, marketOutput(n, ready, "路由验证，不代表业务通过"))
			}
			return results, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"apply_plan", "development", "remediation", "qa_recheck", "acceptance"} {
			if called[id] == blocked {
				t.Fatalf("%s 的阻断或失败修复路径错误，blocked=%t", id, blocked)
			}
		}
	}
}

func TestMarketIterationAcceptsLongStructuredContextThroughTransport(t *testing.T) {
	d := marketDefinition(t)
	calls := 0
	outputs, err := workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
		calls += len(inputs)
		results := []workflow.NodeOutput{}
		for _, input := range inputs {
			n := input.Node
			out := workflow.NodeOutput{Text: "合成研究报告"}
			if n.ID != "market_research" {
				summary := "合成路由验证"
				if n.ID == "product_context" {
					summary = strings.Repeat("有证据的现状", 600)
				}
				out = marketOutput(n, true, summary)
			}
			if err := workflow.ValidateOutput(n, &out); err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			var restored workflow.NodeOutput
			if err := json.Unmarshal(encoded, &restored); err != nil {
				return nil, err
			}
			results = append(results, restored)
		}
		return results, nil
	})
	if err != nil || calls != 25 || outputs["apply_plan"].Skipped || outputs["acceptance"].Text == "" {
		t.Fatalf("长现状输出或结构化传输阻断流程：calls=%d, %v", calls, err)
	}
}

func marketOutput(node workflow.Node, ready bool, summary string) workflow.NodeOutput {
	value := map[string]any{"ready": ready, "status": "complete", "summary": summary, "iteration_complete": node.ID == "acceptance"}
	if node.OutputSchema != nil && node.OutputSchema.Properties["passed"].Type == "boolean" {
		value = map[string]any{"passed": ready, "summary": summary}
	}
	if node.ReportSchema == nil {
		value["evidence"], value["gaps"], value["artifacts"], value["tasks"] = []string{}, []string{}, []string{}, []any{}
	}
	raw, _ := json.Marshal(value)
	out := workflow.NodeOutput{Text: string(raw), ContentRevision: "合成代码摘要"}
	if node.ReportSchema != nil {
		out.ReportIDs, out.ReportSHA256 = []string{uuid.NewString()}, workflow.ReportHash(string(raw))
	}
	return out
}
