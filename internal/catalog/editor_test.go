package catalog

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestWorkflowDraftCanBeIncompleteButCannotPublish(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	item, err := Create(ctx, ns, NewObject{Kind: "workflow", Name: "画布草稿", Spec: Spec{Flow: &Flow{Nodes: []Binding{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(ctx, ns, item.ID, item.Revision); err == nil {
		t.Fatal("空草稿被发布")
	}
	_, version := templateVersion(t, ns)
	flow := &Flow{Nodes: []Binding{{ID: "first", AgentVersion: version.ID, DependsOn: []string{"missing"}}}, Editor: &FlowEditor{Nodes: map[string]NodeDisplay{"first": {Name: "产品规划", X: 120, Y: 240}}}}
	item, err = Save(ctx, ns, item.ID, item.Revision, item.Name, Spec{Flow: flow})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Publish(ctx, ns, item.ID, item.Revision); err == nil {
		t.Fatal("未解决依赖被发布")
	}
	if _, err = Create(ctx, "foreign", NewObject{Kind: "workflow", Name: "越权", Spec: Spec{Flow: flow}}); err == nil {
		t.Fatal("草稿引用了其他工作空间助手")
	}
	flow.Nodes[0].DependsOn = nil
	item, err = Save(ctx, ns, item.ID, item.Revision, item.Name, Spec{Flow: flow})
	if err != nil {
		t.Fatal(err)
	}
	published, err := Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ReadVersion(ctx, ns, published.ID)
	if err != nil || stored.Spec.Flow.Editor.Nodes["first"].Y != 240 {
		t.Fatalf("画布位置丢失: %v", err)
	}
	compiled, err := Compile(ctx, ns, published.ID, project, "检查")
	if err != nil || len(compiled.Nodes) != 1 || compiled.Nodes[0].ID != "first" {
		t.Fatalf("展示信息影响编译: %v", err)
	}
}

func TestWorkflowEditorRejectsInvalidPresentation(t *testing.T) {
	for _, display := range []map[string]NodeDisplay{{"missing": {Name: "不存在"}}, {"first": {X: 1000001}}, {"first": {Name: string(make([]byte, 129))}}} {
		flow := Flow{Nodes: []Binding{{ID: "first", AgentVersion: "11111111-1111-4111-8111-111111111111"}}, Editor: &FlowEditor{Nodes: display}}
		if err := flow.validateDraft(); err == nil {
			t.Fatal("接受非法画布信息")
		}
	}
	flow := Flow{Nodes: []Binding{{ID: "first", AgentVersion: "11111111-1111-4111-8111-111111111111"}}, Budget: &workflow.Budget{ModelCalls: -1}}
	if err := flow.validateDraft(); err == nil {
		t.Fatal("接受负数预算")
	}
}

func TestWorkflowKnownFieldsMustExistAndMatchTypes(t *testing.T) {
	d := workflow.Definition{InputSchema: &workflow.Shape{Type: "object", Properties: map[string]workflow.Shape{"enabled": {Type: "boolean"}}}, Nodes: []workflow.Node{{ID: "a", Bindings: map[string]workflow.FieldBinding{"value": {From: "$input", Pointer: "/deleted", Type: "string"}}}}}
	if validateFields(&d) == nil {
		t.Fatal("已删除字段未被拒绝")
	}
	d.Nodes[0].Bindings["value"] = workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "string"}
	if validateFields(&d) == nil {
		t.Fatal("类型变化未被拒绝")
	}
	d.Nodes[0].Bindings["value"] = workflow.FieldBinding{From: "$input", Pointer: "/enabled", Type: "boolean"}
	if err := validateFields(&d); err != nil {
		t.Fatal(err)
	}
	d.Nodes[0].When = &workflow.Condition{Field: workflow.FieldBinding{From: "$input", Pointer: "/deleted", Type: "boolean"}, Equals: []byte(`true`)}
	if validateFields(&d) == nil {
		t.Fatal("失效执行条件未被拒绝")
	}
}
