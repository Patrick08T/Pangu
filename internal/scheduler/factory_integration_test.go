package scheduler_test

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/factory"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func factoryModel(t *testing.T, ns, project string, calls *atomic.Int32) *catalog.Version {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		reply := factory.Reply{Message: "预览两步流程", Proposal: &catalog.Bundle{Name: "两步写作", Agents: []catalog.AgentChange{{Key: "writer", Name: "写作助手", Node: &workflow.Node{ID: "writer", Executor: "codex", ReadOnly: true, Instruction: "只回复结果", Handoff: testHandoff(), Policy: &workflow.Policy{}}}}, Steps: []catalog.BundleStep{{ID: "draft", Agent: "writer"}, {ID: "review", Agent: "writer", DependsOn: []string{"draft"}}}}}
		raw, _ := json.Marshal(reply)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "factory-test", "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": string(raw)}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(server.Close)
	t.Setenv("PANGU_SECRET_FACTORY_TEST", "test-key")
	grants, _ := json.Marshal([]map[string]any{{"project_id": project, "base_url": server.URL, "secret_ref": "PANGU_SECRET_FACTORY_TEST", "models": []string{"factory-test"}}})
	t.Setenv("PANGU_CONNECTION_GRANTS", string(grants))
	object, err := catalog.Create(context.Background(), ns, catalog.NewObject{Kind: "connection", Name: "隔离模型", Spec: catalog.Spec{Connection: &catalog.Connection{BaseURL: server.URL, SecretRef: "PANGU_SECRET_FACTORY_TEST", Model: "factory-test"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(context.Background(), ns, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestHatchetFactoryConversationGeneratesThenPublishesOnlyOnConfirmation(t *testing.T) {
	_, client := integrationClient(t)
	worker, err := scheduler.NewWorker(client, agent.Execute, agent.Preflight)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	backend := scheduler.NewBackend(client)
	awaitWorker(t, backend)
	project := registerTestProject(t, backend)
	var calls atomic.Int32
	conn := factoryModel(t, backend.Namespace(), project.ID, &calls)
	h := api.New(backend, integrationAccess{})
	var session factory.Session
	request(t, h, "POST", "/factory/sessions", map[string]string{"project_id": project.ID}, 201, &session)
	in := factory.Input{ID: uuid.NewString(), Revision: session.Revision, Message: "创建两步写作流程", Mode: "build", ConnectionVersion: conn.ID}
	var receipt submission.Record
	request(t, h, "POST", "/factory/sessions/"+session.ID+"/messages", in, 202, &receipt)
	run := waitRun(t, h, receipt.RunID)
	if run.Status != "succeeded" {
		t.Fatalf("生成失败: %+v", run)
	}
	if err = backend.ReconcileFactory(context.Background()); err != nil {
		t.Fatal(err)
	}
	request(t, h, "GET", "/factory/sessions/"+session.ID, nil, 200, &session)
	if len(session.Turns) != 1 || session.Turns[0].Status != "ready" || session.Turns[0].Reply.Proposal == nil {
		t.Fatalf("未恢复候选: %+v", session.Turns)
	}
	objects, err := catalog.List(context.Background(), backend.Namespace(), "agent", 0)
	if err != nil || len(objects) != 0 {
		t.Fatal("预览创建资源", err)
	}
	var result catalog.BundleResult
	request(t, h, "POST", "/factory/sessions/"+session.ID+"/apply", map[string]any{"turn_id": in.ID, "revision": session.Revision}, 200, &result)
	if result.Workflow == nil || len(result.Agents) != 1 {
		t.Fatal("批量发布失败")
	}
	again := submission.Record{}
	request(t, h, "POST", "/factory/sessions/"+session.ID+"/messages", in, 202, &again)
	if again.ID != receipt.ID || calls.Load() != 1 {
		t.Fatal("重试调用了第二次模型")
	}
	list, err := backend.ListRuns(context.Background(), scheduler.RunQuery{ProjectID: project.ID, Limit: 100, Since: time.Now().Add(-time.Hour), Until: time.Now().Add(time.Minute)})
	if err != nil || len(list.Items) != 1 {
		t.Fatal("批量创建提前执行业务", err)
	}
	d, err := catalog.Compile(context.Background(), backend.Namespace(), result.Workflow.ID, project.ID, "稍后运行")
	if err != nil || len(d.Nodes) != 2 {
		t.Fatal("生成流程不可编译", err)
	}
}
