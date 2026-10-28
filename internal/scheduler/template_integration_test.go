package scheduler_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestHatchetThreeTemplateFamiliesManualCronAndPinnedUpgrade(t *testing.T) {
	_, client := integrationClient(t)
	model := httptest.NewServer(pmModel(t))
	defer model.Close()
	t.Setenv("OPENAI_BASE_URL", model.URL)
	worker, err := scheduler.NewWorker(client, templateExecutor, func(context.Context, workflow.Definition) workflow.Preflight {
		return workflow.Preflight{Ready: true}
	})
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	b := scheduler.NewBackend(client)
	awaitWorker(t, b)
	project := registerTestProject(t, b)
	plans := map[string]string{}
	for _, name := range []string{"code-change", "code-inspection", "product-research"} {
		d := boundTemplate(t, b, project.ID, name)
		receipt, err := b.Submit(context.Background(), *d, "manual-"+name)
		if err != nil {
			t.Fatal(err)
		}
		assertTemplateRun(t, b, receipt.RunID, d.VersionID)
		plan, err := b.CreateCron(context.Background(), workflow.Cron{Name: name, Expression: "* * * * *", Workflow: *d})
		if err != nil || !plan.Ready {
			t.Fatalf("模板计划未注册: %+v %v", plan, err)
		}
		t.Cleanup(func() { _ = b.DeleteCron(context.Background(), plan.ID) })
		plans[plan.ID] = d.VersionID
	}
	for id, version := range plans {
		run := awaitTemplateCron(t, b, id)
		if err := b.DeleteCron(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		assertTemplateRun(t, b, run, version)
	}
}

// 模型响应使用本地夹具；编排、Cron、数据库、Git 与产品经理工具链是真实实现。
func templateExecutor(ctx context.Context, input workflow.NodeInput) (workflow.NodeOutput, error) {
	if input.Node.Role == "product_manager" {
		return agent.Execute(ctx, input)
	}
	if !input.Node.ReadOnly {
		if err := os.WriteFile(filepath.Join(input.Workspace, "template-proof.txt"), []byte("模板绑定版本验证"), 0600); err != nil {
			return workflow.NodeOutput{}, err
		}
	}
	return workflow.NodeOutput{Text: input.Node.Instruction}, nil
}

func boundTemplate(t *testing.T, b scheduler.Backend, project, name string) *workflow.Definition {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var input catalog.NewObject
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	ctx, ns := context.Background(), b.Namespace()
	template, err := catalog.Create(ctx, ns, input)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := catalog.Publish(ctx, ns, template.ID, template.Revision)
	if err != nil {
		t.Fatal(err)
	}
	connection := ""
	if input.Spec.Agent.Executor == "eino" {
		connection = templateConnection(t, ns, project)
	}
	bound, err := catalog.Bind(ctx, ns, v1.ID, project, name, connection)
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, ns, bound.ID, bound.Revision)
	if err != nil {
		t.Fatal(err)
	}
	original := template.Spec.Agent.Instruction
	template.Spec.Agent.Instruction = "后续版本，不应进入旧绑定或计划"
	updated, err := catalog.Save(ctx, ns, template.ID, template.Revision, template.Name, template.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Publish(ctx, ns, updated.ID, updated.Revision); err != nil {
		t.Fatal(err)
	}
	brief, _ := json.Marshal(workflow.ResearchBrief{Question: "规划任务台", Audience: "小型团队", Market: "中国", TimeScope: "2026", Context: "人工登记", Constraints: "工期未知", Documents: []string{"README.md"}})
	d, err := catalog.Compile(ctx, ns, v.ID, string(brief))
	if err != nil || d.Nodes[0].Instruction != original {
		t.Fatalf("模板升级污染了已绑定版本: %+v %v", d, err)
	}
	return d
}

func templateConnection(t *testing.T, namespace, project string) string {
	t.Helper()
	ctx := context.Background()
	endpoint := os.Getenv("OPENAI_BASE_URL")
	grants, _ := json.Marshal([]map[string]any{{"project_id": project, "base_url": endpoint, "secret_ref": "PANGU_SECRET_TEMPLATE_TEST", "models": []string{"research"}}})
	t.Setenv("PANGU_CONNECTION_GRANTS", string(grants))
	t.Setenv("PANGU_SECRET_TEMPLATE_TEST", "local-template-fixture")
	object, err := catalog.Create(ctx, namespace, catalog.NewObject{Kind: "connection", ProjectID: project, Name: "模板本地模型", Spec: catalog.Spec{Connection: &catalog.Connection{BaseURL: endpoint, Model: "research", SecretRef: "PANGU_SECRET_TEMPLATE_TEST"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, namespace, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func assertTemplateRun(t *testing.T, b scheduler.Backend, id, version string) {
	t.Helper()
	run := waitRun(t, api.New(b, integrationAccess{}), id)
	if run.Definition == nil || run.Definition.VersionID != version || len(run.Attempts) != 1 || len(run.Nodes) != 1 {
		t.Fatalf("模板运行版本或尝试关联错误: %+v", run)
	}
}

func awaitTemplateCron(t *testing.T, b scheduler.Backend, id string) string {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		fires, err := b.CronFires(context.Background(), id, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, fire := range fires {
			if fire.RequestID == "" {
				continue
			}
			request, err := b.GetRequest(context.Background(), fire.RequestID)
			if err == nil && request.RunID != "" {
				return request.RunID
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("真实 Cron 没有触发模板运行")
	return ""
}
