package catalog

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

const qianliyanExample = "../../examples/qianliyan/"

func readQianliyanExample(t *testing.T) (Bundle, string) {
	t.Helper()
	file, err := os.Open(qianliyanExample + "workflow.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var bundle Bundle
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	brief, err := os.ReadFile(qianliyanExample + "brief.json")
	if err != nil {
		t.Fatal(err)
	}
	return bundle, string(brief)
}

func TestQianliyanExampleIsPortable(t *testing.T) {
	bundle, input := readQianliyanExample(t)
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	if bundle.ObjectID != "" || len(bundle.Agents) != 3 || len(bundle.Steps) != 3 {
		t.Fatal("示例必须独立创建一条三节点工作流")
	}
	brief, err := workflow.ParseBrief(input, "research")
	if err != nil || len(brief.URLs) == 0 {
		t.Fatal("示例需包含可用调研输入和明确授权的公开网址", err)
	}
	for index, id := range []string{"collect", "feishu", "wechat"} {
		step, agent := bundle.Steps[index], bundle.Agents[index]
		if step.ID != id || step.Agent != id || agent.Key != id || agent.Node == nil {
			t.Fatalf("节点 %s 缺少独立角色", id)
		}
		if agent.ObjectID != "" || agent.VersionID != "" || agent.Node.ConnectionVersion != "" || agent.Node.AgentVersion != "" {
			t.Fatalf("节点 %s 含环境绑定", id)
		}
		if err := agent.Node.Validate(); err != nil {
			t.Fatal(err)
		}
		if index == 0 && (agent.Node.Role != "product_manager" || agent.Node.Mode != "research" || len(step.DependsOn) > 0) {
			t.Fatal("采集需使用调研角色")
		}
		if index > 0 && (!slices.Equal(step.DependsOn, []string{"collect"}) || agent.Node.OutputSchema == nil) {
			t.Fatalf("节点 %s 缺少采集依赖或交付输出结构", id)
		}
	}
}

func TestQianliyanExamplePublishesAndCompiles(t *testing.T) {
	namespace, project, _ := catalogDatabase(t)
	ctx := context.Background()
	bundle, input := readQianliyanExample(t)
	model := bundle.Agents[0].Node.Model
	conn, err := Create(ctx, namespace, NewObject{Kind: "connection", Name: "示例模型连接", Spec: Spec{Connection: &Connection{
		Provider: "openai", API: "responses", ReasoningEffort: "high",
		Model: model, BaseURL: "https://example.org/v1", SecretRef: "PANGU_SECRET_EXAMPLE_TEST",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := Publish(ctx, namespace, conn.ID, conn.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range bundle.Agents {
		agent.Node.ConnectionVersion = connection.ID
	}
	db, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	result, err := ApplyBundleTx(ctx, tx, namespace, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	definition, err := Compile(ctx, namespace, result.Workflow.ID, project, input)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := definition.Layers()
	if err != nil || len(layers) != 2 || len(layers[0]) != 1 || len(layers[1]) != 2 {
		t.Fatal("示例需在采集后并行准备两路交付", err)
	}
	for _, node := range layers[1] {
		if node.ConnectionVersion != connection.ID || node.AgentVersion == "" {
			t.Fatalf("节点 %s 未固定到新发布版本和共享模型连接", node.ID)
		}
	}
}
