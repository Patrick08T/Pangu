package scheduler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/agent"
	"github.com/Patrick08T/Pangu/internal/api"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type modelServer struct {
	mu    sync.Mutex
	calls map[string]int
}

func (m *modelServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	m.mu.Lock()
	m.calls[request.Model]++
	count := m.calls[request.Model]
	m.mu.Unlock()
	if request.Model == "recover" && count == 1 {
		<-r.Context().Done()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if request.Model == "flaky" && count == 1 {
		w.WriteHeader(503)
		_, _ = fmt.Fprint(w, `{"error":{"message":"temporary model failure","type":"server_error"}}`)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":"local","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}]}`, request.Model+" output")
}

func (m *modelServer) count(model string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[model]
}

func integrationSetup(t *testing.T) (*modelServer, scheduler.Backend, http.Handler) {
	m, client := integrationClient(t)
	worker, err := scheduler.NewWorker(client, agent.Execute)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	b := scheduler.NewBackend(client)
	return m, b, api.New(b)
}

func integrationClient(t *testing.T) (*modelServer, *hatchet.Client) {
	t.Helper()
	if os.Getenv("PANGU_INTEGRATION") != "1" {
		t.Skip("设置 PANGU_INTEGRATION=1 并连接本地 Hatchet 后执行")
	}
	m := &modelServer{calls: map[string]int{}}
	server := httptest.NewServer(m)
	t.Cleanup(server.Close)
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_API_KEY", "local-integration")
	t.Setenv("HATCHET_CLIENT_NAMESPACE", fmt.Sprintf("test_%d", time.Now().UnixNano()))
	client, err := hatchet.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return m, client
}

func request(t *testing.T, h http.Handler, method, path string, value any, status int, out any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
}

func waitRun(t *testing.T, h http.Handler, id string) scheduler.RunDetails {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		var out scheduler.RunDetails
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/runs/"+id, nil))
		if w.Code == 404 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if w.Code != 200 {
			t.Fatalf("查询运行失败: %d %s", w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Run.Run.Status == "COMPLETED" {
			return out
		}
		if out.Run.Run.Status == "FAILED" || out.Run.Run.Status == "CANCELLED" {
			t.Fatalf("run failed: status=%s error=%v", out.Run.Run.Status, out.Run.Run.ErrorMessage)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("等待 Hatchet 运行超时")
	return scheduler.RunDetails{}
}

func TestHatchetNodeRetry(t *testing.T) {
	m, _, h := integrationSetup(t)
	d := workflow.Definition{Input: "integration", Nodes: []workflow.Node{
		{ID: "source", Model: "source", Instruction: "source"},
		{ID: "flaky", Model: "flaky", Instruction: "flaky", DependsOn: []string{"source"}},
		{ID: "final", Model: "final", Instruction: "final", DependsOn: []string{"flaky"}},
	}}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	out := waitRun(t, h, accepted.RunID)
	if m.count("source") != 1 || m.count("flaky") != 2 || m.count("final") != 1 {
		t.Fatalf("calls=%v", m.calls)
	}
	if len(out.Nodes) != 3 {
		t.Fatalf("expected 3 independent child runs, got %d", len(out.Nodes))
	}
	for _, node := range out.Nodes {
		if node.Status != "COMPLETED" {
			t.Fatalf("node did not complete: %+v", node)
		}
	}
	t.Logf("真实 Hatchet 节点重试通过，运行 ID=%s", accepted.RunID)
}

func TestHatchetCron(t *testing.T) {
	m, _, h := integrationSetup(t)
	c := workflow.Cron{Name: fmt.Sprintf("integration-%d", time.Now().UnixNano()), Expression: "* * * * *", Workflow: workflow.Definition{
		Input: "cron", Nodes: []workflow.Node{{ID: "cron", Model: "cron", Instruction: "cron"}},
	}}
	var created rest.CronWorkflows
	request(t, h, "POST", "/crons", c, 201, &created)
	t.Cleanup(func() { request(t, h, "DELETE", "/crons/"+created.Metadata.Id, nil, 204, nil) })
	var listed rest.CronWorkflowsList
	request(t, h, "GET", "/crons", nil, 200, &listed)
	if listed.Rows == nil || len(*listed.Rows) == 0 {
		t.Fatal("Cron not listed")
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) && m.count("cron") == 0 {
		time.Sleep(500 * time.Millisecond)
	}
	if m.count("cron") == 0 {
		t.Fatal("Cron did not trigger")
	}
	t.Log("真实 Hatchet Cron 已触发 Agent 执行")
}

func TestHatchetWorkerRecovery(t *testing.T) {
	if os.Getenv("PANGU_TEST_BINARY") == "" {
		t.Skip("设置 PANGU_TEST_BINARY 指向已构建的 Pangu 可执行文件")
	}
	m, client := integrationClient(t)
	h := api.New(scheduler.NewBackend(client))
	first := startProcess(t)
	d := workflow.Definition{Input: "restart", Nodes: []workflow.Node{
		{ID: "source", Model: "source", Instruction: "source"},
		{ID: "recover", Model: "recover", Instruction: "recover", DependsOn: []string{"source"}},
		{ID: "final", Model: "final", Instruction: "final", DependsOn: []string{"recover"}},
	}}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && m.count("recover") == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	if m.count("recover") != 1 {
		t.Fatal("节点未启动，无法执行崩溃测试")
	}
	if err := first.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	startProcess(t)
	_ = waitRun(t, h, accepted.RunID)
	if m.count("source") != 1 || m.count("recover") != 2 || m.count("final") != 1 {
		t.Fatalf("恢复调用次数：source=%d recover=%d final=%d", m.count("source"), m.count("recover"), m.count("final"))
	}
	t.Logf("强制终止 Worker 后恢复成功，完成节点未重跑，运行 ID=%s", accepted.RunID)
}

func startProcess(t *testing.T) *exec.Cmd {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "worker.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Getenv("PANGU_TEST_BINARY"), "-mode", "worker")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = logFile.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Logf("Worker 日志: %s", data)
		}
	})
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), "Pangu Worker 已就绪") {
			return cmd
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Worker 未在期限内完成注册")
	return nil
}

func TestHatchetWideLargeInput(t *testing.T) {
	m, _, h := integrationSetup(t)
	d := workflow.Definition{Input: strings.Repeat("x", 200*1024)}
	for i := 0; i < 32; i++ {
		d.Nodes = append(d.Nodes, workflow.Node{ID: fmt.Sprintf("node%d", i), Model: "wide", Instruction: "test"})
	}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	out := waitRun(t, h, accepted.RunID)
	if len(out.Nodes) != 32 || m.count("wide") != 32 {
		t.Fatalf("nodes=%d calls=%d", len(out.Nodes), m.count("wide"))
	}
	firstID := accepted.RunID
	d = workflow.Definition{Input: "later", Nodes: []workflow.Node{{ID: "later", Model: "later", Instruction: "later"}}}
	request(t, h, "POST", "/runs", d, 202, &accepted)
	_ = waitRun(t, h, accepted.RunID)
	request(t, h, "GET", "/runs/"+firstID, nil, 200, &out)
	if len(out.Nodes) != 32 {
		t.Fatalf("新运行后历史节点不完整: got=%d", len(out.Nodes))
	}
	for _, node := range out.Nodes {
		if node.Input == nil || node.Output == nil {
			t.Fatal("节点详情缺少输入或输出")
		}
	}
}

func TestHatchetRejectBeforeRegistration(t *testing.T) {
	_, client := integrationClient(t)
	h := api.New(scheduler.NewBackend(client))
	d := workflow.Definition{Input: "unregistered", Nodes: []workflow.Node{{ID: "test", Model: "test", Instruction: "test"}}}
	request(t, h, "POST", "/runs", d, 503, nil)
}
