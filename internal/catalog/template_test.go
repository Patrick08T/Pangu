package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestFlowStepTaskSchemaAndReadOnlySurvivePublishing(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	_, role := templateVersion(t, ns)
	item, _ := flowTemplate(t, ns, role.ID, role.ID)
	shape := &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"ready": {Type: "boolean"}}, Required: []string{"ready"}}
	item.Spec.Flow.Nodes[0].Task, item.Spec.Flow.Nodes[0].OutputSchema, item.Spec.Flow.Nodes[0].ReadOnly = "读取基线", shape, true
	item.Spec.Flow.Nodes[1].Task = "执行单一优化"
	item.Spec.Flow.Nodes[1].DependsOn = []string{"a"}
	item.Spec.Flow.Nodes[1].When = &workflow.Condition{Field: workflow.FieldBinding{From: "a", Pointer: "/ready", Type: "boolean"}, Equals: []byte("true")}
	item.Spec.Flow.Nodes[1].Approval = &workflow.Approval{TimeoutSeconds: 1800}
	item, err := Save(ctx, ns, item.ID, item.Revision, item.Name, item.Spec)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, v.ID, project, "任务")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Nodes[0].ReadOnly || d.Nodes[1].ReadOnly || !strings.Contains(d.Nodes[0].Instruction, "读取基线") || !sameJSON(d.Nodes[0].OutputSchema, shape) || d.Nodes[1].Approval == nil {
		t.Fatal("步骤配置丢失或去重串用了权限")
	}
	if err := CheckDefinition(ctx, ns, *d); err != nil {
		t.Fatal(err)
	}
	d.Nodes[0].ReadOnly = false
	if CheckDefinition(ctx, ns, *d) == nil {
		t.Fatal("执行时允许放宽已发布的权限")
	}

}

func TestTemplateGraphValidationDoesNotRelaxExecution(t *testing.T) {
	d := workflow.Definition{Nodes: []workflow.Node{{ID: "a", Executor: "codex", Instruction: "只读"}}}
	if _, err := d.TemplateLayers(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Layers(); err == nil {
		t.Fatal("未绑定项目也能执行")
	}
	d.Nodes[0].DependsOn = []string{"a"}
	if _, err := d.TemplateLayers(); err == nil {
		t.Fatal("模板接受环路")
	}
	for _, step := range []Binding{{Task: strings.Repeat("a", 8193)}, {OutputSchema: &workflow.Shape{Type: "string"}}} {
		if step.validateStep() == nil {
			t.Fatal("接受非法步骤参数")
		}
	}
}

func flowTemplate(t *testing.T, ns string, agents ...string) (*Object, *Version) {
	t.Helper()
	flow := &Flow{}
	for i, id := range agents {
		flow.Nodes = append(flow.Nodes, Binding{ID: string(rune('a' + i)), AgentVersion: id})
	}
	item, err := Create(context.Background(), ns, NewObject{Kind: "workflow", Name: "跨项目性能流程", Spec: Spec{Flow: flow}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(context.Background(), ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return item, v
}

func TestSharedFlowRejectsNestedFlowAndArchivedNewReferences(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	role, v := templateVersion(t, ns)
	flow, published := flowTemplate(t, ns, v.ID)
	if _, err := Save(ctx, ns, role.ID, role.Revision, role.Name, flow.Spec); err == nil {
		t.Fatal("角色接受工作流配置")
	}
	nested := NewObject{Kind: "workflow", Name: "嵌套流程", Spec: Spec{Flow: &Flow{Nodes: []Binding{{ID: "a", AgentVersion: published.ID}}}}}
	if _, err := Create(ctx, ns, nested); err == nil {
		t.Fatal("接受嵌套工作流")
	}
	if err := Archive(ctx, ns, role.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(ctx, ns, flow.ID, flow.Revision, flow.Name, flow.Spec); err == nil {
		t.Fatal("允许新引用已归档角色")
	}
	if _, err := Compile(ctx, ns, published.ID, project, "任务"); err != nil {
		t.Fatal("归档破坏历史引用", err)
	}
}
