package catalog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestConnectionIsProjectScopedAndSecretsReadFromSource(t *testing.T) {
	ns, project, other := catalogDatabase(t)
	grants, _ := json.Marshal([]credentialGrant{{ProjectID: project, BaseURL: "https://model.example/v1", SecretRef: "PANGU_SECRET_TEST", Models: []string{"test"}}})
	t.Setenv("PANGU_CONNECTION_GRANTS", string(grants))
	ctx := context.Background()
	item, err := Create(ctx, ns, NewObject{Kind: "connection", Name: "模型", ProjectID: project, Spec: Spec{Connection: &Connection{BaseURL: "https://model.example/v1", Model: "test", SecretRef: "PANGU_SECRET_TEST"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_SECRET_TEST", "first-private-value")
	_, key, err := ResolveConnection(ctx, project, v.ID)
	if err != nil || key != "first-private-value" {
		t.Fatalf("凭据解析: %q %v", key, err)
	}
	t.Setenv("PANGU_SECRET_TEST", "second-private-value")
	_, key, err = ResolveConnection(ctx, project, v.ID)
	if err != nil || key != "second-private-value" {
		t.Fatal("读取了缓存凭据")
	}
	if _, _, err := ResolveConnection(ctx, other, v.ID); err == nil {
		t.Fatal("接受跨项目连接")
	}
	node := workflow.Node{ID: "agent", Executor: "eino", Model: "test", Instruction: "任务", ConnectionVersion: v.ID}
	if _, err := Create(ctx, ns, NewObject{Kind: "template", Name: "泄漏项目连接", Spec: Spec{Agent: &node}}); err == nil {
		t.Fatal("共享模板携带项目连接")
	}
	if _, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "跨项目", ProjectID: other, Spec: Spec{Agent: &node}}); err == nil {
		t.Fatal("Agent 接受其他项目连接")
	}
	if err := Archive(ctx, ns, item.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveConnection(ctx, project, v.ID); err == nil {
		t.Fatal("归档连接仍可新调用")
	}
}

func TestWorkflowPinsAgentVersionAndRejectsCrossProjectReferences(t *testing.T) {
	ns, project, other := catalogDatabase(t)
	ctx := context.Background()
	_, template := templateVersion(t, ns)
	agent, err := Bind(ctx, ns, template.ID, project, "项目 Agent", "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, agent.ID, agent.Revision)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{Flow: &Flow{Nodes: []Binding{{ID: "first", AgentVersion: v.ID}, {ID: "second", AgentVersion: v.ID, DependsOn: []string{"first"}}}}}
	if _, err := Create(ctx, ns, NewObject{Kind: "workflow", Name: "跨项目流程", ProjectID: other, Spec: spec}); err == nil {
		t.Fatal("跨项目流程引用通过")
	}
	flow, err := Create(ctx, ns, NewObject{Kind: "workflow", Name: "固定版本", ProjectID: project, Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	flowVersion, err := Publish(ctx, ns, flow.ID, flow.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := Archive(ctx, ns, agent.ID, true); err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, flowVersion.ID, "运行输入")
	if err != nil || d.Nodes[0].AgentVersion != v.ID || d.Nodes[1].Instruction != "检查项目" {
		t.Fatalf("历史引用失效: %+v %v", d, err)
	}
}

func TestRawDefinitionCannotForgePublishedVersion(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	_, template := templateVersion(t, ns)
	a, err := Bind(ctx, ns, template.ID, project, "固定 Agent", "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, v.ID, "输入")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckDefinition(ctx, ns, *d); err != nil {
		t.Fatal(err)
	}
	d.Nodes[0].Instruction = "伪装成旧版本的新指令"
	if err := CheckDefinition(ctx, ns, *d); err == nil {
		t.Fatal("伪造版本快照被接受")
	}
	d.VersionID = ""
	if err := CheckDefinition(ctx, ns, *d); err == nil {
		t.Fatal("伪造 Agent 版本被接受")
	}
}

func TestStandaloneAgentCannotPublishDependencyBindings(t *testing.T) {
	item := NewObject{Kind: "template", Name: "非法绑定", Spec: Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Instruction: "任务", Bindings: map[string]workflow.FieldBinding{"x": {From: "missing", Pointer: "/x", Type: "string"}}}}}
	if err := item.Validate(); err == nil {
		t.Fatal("独立 Agent 接受不存在的上游绑定")
	}
}
