package api

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/scheduler"
	"github.com/Patrick08T/Pangu/internal/trace"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

type debugBackend struct{ feedbackBackend }

func (b *debugBackend) GetRun(ctx context.Context, id string) (*scheduler.RunDetails, error) {
	run, err := b.feedbackBackend.GetRun(ctx, id)
	run.StopConfirmed = false
	run.Status = "running"
	return run, err
}
func debugAPI(t *testing.T) (http.Handler, string, string, string) {
	t.Helper()
	_, ns, project, run, token := feedbackAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var owner string
	if err = conn.QueryRow(ctx, "SELECT created_by::text FROM pangu.submissions WHERE run_id=$1", run).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	h := New(&debugBackend{feedbackBackend{authorizedBackend: authorizedBackend{namespace: ns}, project: project, owner: owner}}, identity.Service{Namespace: ns})
	return h, project, run, token
}

func TestDebugRequiresCurrentAuthorityRevisionAndExplicitCommand(t *testing.T) {
	h, project, run, token := debugAPI(t)
	path := "/runs/" + run + "/debug"
	if w := callAuthorized(h, "POST", path, `{"action":"pause","revision":0}`, token); w.Code != 403 {
		t.Fatalf("viewer 可暂停: %d %s", w.Code, w.Body.String())
	}
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='operator' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w := callAuthorized(h, "POST", path, `{"action":"pause"}`, token); w.Code != 400 {
		t.Fatalf("缺少版本仍被接受: %d %s", w.Code, w.Body.String())
	}
	w := callAuthorized(h, "POST", path, `{"action":"pause","revision":0}`, token)
	var state execution.DebugState
	if json.Unmarshal(w.Body.Bytes(), &state) != nil || w.Code != 200 || !state.Paused || state.Revision != 1 {
		t.Fatalf("暂停失败: %d %s", w.Code, w.Body.String())
	}
	if w = callAuthorized(h, "POST", path, `{"action":"resume","revision":0}`, token); w.Code != 409 {
		t.Fatalf("旧版本覆盖状态: %d", w.Code)
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET enabled=false WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w = callAuthorized(h, "POST", path, `{"action":"resume","revision":1}`, token); w.Code != 403 {
		t.Fatalf("撤权后可继续: %d", w.Code)
	}
}

func TestTraceReadFilterAndExportRespectProjectAuthority(t *testing.T) {
	h, project, run, token := debugAPI(t)
	ctx := trace.WithRun(context.Background(), run, "writer", uuid.NewString(), 1)
	_, span, err := trace.Start(ctx, "tool", "read_project", "token=private-credential")
	if err != nil {
		t.Fatal(err)
	}
	if err = span.End("资料结果", nil); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"/trace?node=writer", "/trace.ndjson?node=writer", "/activity"} {
		w := callAuthorized(h, "GET", "/runs/"+run+suffix, "", token)
		if w.Code != 200 {
			t.Fatalf("读取失败: %d %s", w.Code, w.Body.String())
		}
	}
	if w := callAuthorized(h, "GET", "/runs/"+run+"/trace?limit=101", "", token); w.Code != 400 {
		t.Fatalf("分页未校验: %d", w.Code)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET enabled=false WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w := callAuthorized(h, "GET", "/runs/"+run+"/trace.ndjson", "", token); w.Code != 403 {
		t.Fatalf("撤权后可导出: %d", w.Code)
	}
	if w := callAuthorized(h, "GET", "/runs/"+run+"/activity", "", token); w.Code != 403 {
		t.Fatalf("撤权后可读取当前活动: %d", w.Code)
	}
}

func TestRunActivityKeepsLongOperationBeyondTracePage(t *testing.T) {
	h, _, run, token := debugAPI(t)
	ctx := context.Background()
	item := execution.Attempt{RunID: run, NodeID: "writer", NodeRunID: uuid.NewString()}
	if err := execution.Begin(ctx, &item); err != nil {
		t.Fatal(err)
	}
	ctx = trace.WithRun(ctx, run, item.NodeID, item.NodeRunID, item.Attempt)
	ctx, _, err := trace.Start(ctx, "node", "writer", "")
	if err != nil {
		t.Fatal(err)
	}
	_, active, err := trace.Start(ctx, "tool", "long_command", "token=private-credential")
	if err != nil {
		t.Fatal(err)
	}
	for range 55 {
		_, ended, err := trace.Start(ctx, "tool", "completed_command", "")
		if err != nil {
			t.Fatal(err)
		}
		if err = ended.End("完成", nil); err != nil {
			t.Fatal(err)
		}
	}
	path := "/runs/" + run + "/activity"
	w := callAuthorized(h, "GET", path, "", token)
	var events []trace.Event
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &events) != nil || len(events) != 1 || events[0].Name != "long_command" {
		t.Fatalf("长调用被历史记录遮蔽: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "private-credential") {
		t.Fatal("当前活动泄露凭据")
	}
	if err = active.End("完成", nil); err != nil {
		t.Fatal(err)
	}
	w = callAuthorized(h, "GET", path, "", token)
	if json.Unmarshal(w.Body.Bytes(), &events) != nil || len(events) != 1 || events[0].Kind != "node" {
		t.Fatalf("已结束的工具仍显示活动: %s", w.Body.String())
	}
	item.Status = "succeeded"
	if err = execution.Finish(ctx, item); err != nil {
		t.Fatal(err)
	}
	w = callAuthorized(h, "GET", path, "", token)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("已结束尝试仍活动: %s", w.Body.String())
	}
	if w = callAuthorized(h, "GET", path+"?node=writer", "", token); w.Code != 400 {
		t.Fatalf("接受未知参数: %d", w.Code)
	}
}
