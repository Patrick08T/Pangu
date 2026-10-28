package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func performanceDefinition(t *testing.T) workflow.Definition {
	t.Helper()
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	raw, err := os.ReadFile("../../examples/workflows/game-backend-performance.json")
	if err != nil {
		t.Fatal(err)
	}
	var item NewObject
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{"scope": "pm-planner", "baseline": "game-performance-engineer", "implement": "dev-engineer", "regression": "qa-engineer", "remeasure": "game-performance-engineer", "security": "game-security-engineer", "acceptance": "acceptance-reviewer", "handover": "game-release-engineer"}
	versions := map[string]string{}
	for i, node := range item.Spec.Flow.Nodes {
		key := roles[node.ID]
		if versions[key] == "" {
			raw, err := os.ReadFile("../../examples/templates/" + key + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var role NewObject
			if err := json.Unmarshal(raw, &role); err != nil {
				t.Fatal(err)
			}
			obj, err := Create(ctx, ns, role)
			if err != nil {
				t.Fatal(err)
			}
			v, err := Publish(ctx, ns, obj.ID, obj.Revision)
			if err != nil {
				t.Fatal(err)
			}
			versions[key] = v.ID
		}
		item.Spec.Flow.Nodes[i].AgentVersion = versions[key]
	}
	obj, err := Create(ctx, ns, item)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, obj.ID, obj.Revision)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]string{}
	for field := range item.Spec.Flow.Parameters.Properties {
		input[field] = "合成路由测试输入，不代表实际授权"
	}
	data, _ := json.Marshal(input)
	d, err := Compile(ctx, ns, v.ID, project, string(data))
	if err != nil {
		t.Fatal(err)
	}
	return *d
}

func TestPerformanceBlueprintGatesAndBlockedPaths(t *testing.T) {
	d := performanceDefinition(t)
	layers, err := d.Layers()
	if err != nil || len(layers) != 8 || len(d.Nodes) != 8 {
		t.Fatal("流程不是八阶段串行", err)
	}
	if d.Nodes[2].Approval == nil || d.Nodes[2].Approval.TimeoutSeconds != 1800 || d.Budget.CodexSessions != 8 {
		t.Fatal("缺少审批或明确会话预算")
	}
	for i, n := range d.Nodes {
		if n.EffectivePolicy().Retries != 0 || n.ReadOnly != (i != 2 && i != 3) {
			t.Fatalf("%s 的执行策略错误", n.ID)
		}
	}
	for stop := -1; stop < len(d.Nodes); stop++ {
		calls := 0
		outputs, err := workflow.Run(d, func(inputs []workflow.NodeInput) ([]workflow.NodeOutput, error) {
			if len(inputs) != 1 {
				t.Fatal("出现了并行测量")
			}
			ready := calls != stop
			calls++
			return []workflow.NodeOutput{{Text: fmt.Sprintf(`{"ready":%t,"summary":"合成结果","evidence":[],"gaps":[]}`, ready)}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		want := len(d.Nodes)
		if stop >= 0 {
			want = stop + 1
		}
		if calls != want {
			t.Fatalf("阻塞阶段 %d 后仍执行: %d", stop, calls)
		}
		for i, n := range d.Nodes {
			if outputs[n.ID].Skipped != (stop >= 0 && i > stop) {
				t.Fatalf("%s 跳过状态错误", n.ID)
			}
		}
	}
	_, err = workflow.Run(d, func([]workflow.NodeInput) ([]workflow.NodeOutput, error) {
		return []workflow.NodeOutput{{Text: `{"ready":true}`}}, nil
	})
	if err == nil {
		t.Fatal("缺失交接字段仍继续执行")
	}
}
