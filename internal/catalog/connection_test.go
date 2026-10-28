package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestCodexConnectionIsSharedAcrossProjects(t *testing.T) {
	ns, project, other := catalogDatabase(t)
	ctx := context.Background()
	var spec Spec
	if err := json.Unmarshal([]byte(`{"connection":{"provider":"codex"}}`), &spec); err != nil {
		t.Fatal(err)
	}
	item, err := Create(ctx, ns, NewObject{Kind: "connection", Name: "Worker Codex", Spec: spec})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Publish(ctx, ns, item.ID, item.Revision)
	if err != nil {
		t.Fatal(err)
	}
	c, key, err := ResolveConnection(ctx, project, v.ID)
	if err != nil || key != "" || c.Model != "" {
		t.Fatalf("账号连接解析失败: %v", err)
	}
	if _, _, err := ResolveConnection(ctx, other, v.ID); err != nil {
		t.Fatal("共享连接无法跨项目使用")
	}
	_, foreign, _ := catalogDatabase(t)
	if _, _, err := ResolveConnection(ctx, foreign, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("连接泄漏到其他工作空间", err)
	}
	node := workflow.Node{ID: "pm", Executor: "eino", Handoff: testHandoff(), Instruction: "规划", ConnectionVersion: v.ID}
	a, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "产品经理", Spec: Spec{Agent: &node}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(ctx, ns, a.ID, a.Revision); err != nil {
		t.Fatal(err)
	}
	if err := Archive(ctx, ns, item.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveConnection(ctx, project, v.ID); !errors.Is(err, ErrArchived) {
		t.Fatal("读取了已归档连接", err)
	}
}

func TestConnectionRejectsMixedCredentialsAndUnknownProvider(t *testing.T) {
	for _, raw := range []string{
		`{"provider":"codex","base_url":"https://example.com"}`,
		`{"provider":"codex","secret_ref":"PANGU_SECRET_TEST"}`,
		`{"provider":"codex","model":" "}`,
		`{"provider":"unknown","base_url":"https://example.com","model":"test","secret_ref":"PANGU_SECRET_TEST"}`,
	} {
		var c Connection
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatal(err)
		}
		if c.Validate() == nil {
			t.Fatalf("接受非法连接: %s", raw)
		}
	}
}

func TestSharedConnectionReadsSecretsFromSourceWithoutProjectGrants(t *testing.T) {
	ns, project, other := catalogDatabase(t)
	t.Setenv("PANGU_CONNECTION_GRANTS", "")
	ctx := context.Background()
	item, err := Create(ctx, ns, NewObject{Kind: "connection", Name: "模型", Spec: Spec{Connection: &Connection{BaseURL: "https://model.example/v1", Model: "test", SecretRef: "PANGU_SECRET_TEST"}}})
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
	if _, _, err := ResolveConnection(ctx, other, v.ID); err != nil {
		t.Fatal("共享连接无法跨项目使用")
	}
	node := workflow.Node{ID: "agent", Executor: "eino", Model: "test", Handoff: testHandoff(), Instruction: "任务", ConnectionVersion: v.ID}
	if _, err := Create(ctx, ns, NewObject{Kind: "agent", Name: "共享角色", Spec: Spec{Agent: &node}}); err != nil {
		t.Fatal("Agent 无法使用共享连接")
	}
	if err := Archive(ctx, ns, item.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveConnection(ctx, project, v.ID); err == nil {
		t.Fatal("归档连接仍可新调用")
	}
}

func TestWorkflowPinsSharedAgentVersion(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	agent, _ := templateVersion(t, ns)
	v, err := Publish(ctx, ns, agent.ID, agent.Revision)
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{Flow: &Flow{Nodes: []Binding{{ID: "first", AgentVersion: v.ID}, {ID: "second", AgentVersion: v.ID, DependsOn: []string{"first"}}}}}
	flow, err := Create(ctx, ns, NewObject{Kind: "workflow", Name: "固定版本", Spec: spec})
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
	d, err := Compile(ctx, ns, flowVersion.ID, project, "运行输入")
	if err != nil || d.Nodes[0].AgentVersion != v.ID || d.Nodes[1].Instruction != "检查项目" {
		t.Fatalf("历史引用失效: %+v %v", d, err)
	}
}

func TestRawDefinitionCannotForgePublishedVersion(t *testing.T) {
	ns, project, _ := catalogDatabase(t)
	ctx := context.Background()
	a, _ := templateVersion(t, ns)
	v, err := Publish(ctx, ns, a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compile(ctx, ns, v.ID, project, "输入")
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
	item := NewObject{Kind: "agent", Name: "非法绑定", Spec: Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Handoff: testHandoff(), Instruction: "任务", Bindings: map[string]workflow.FieldBinding{"x": {From: "missing", Pointer: "/x", Type: "string"}}}}}
	if err := item.Validate(); err == nil {
		t.Fatal("独立 Agent 接受不存在的上游绑定")
	}
}
