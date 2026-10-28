package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/webhook"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func githubCall(h http.Handler, id, event, body, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/hooks/github/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", event)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

func TestGithubWebhookIsAtomicIdempotentAndRevocable(t *testing.T) {
	h, ns, project, _, token := authorizationAPI(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET role='manager' WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	object, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: "任务执行", Spec: catalog.Spec{Agent: &workflow.Node{ID: "code", Executor: "codex", Handoff: testHandoff(), Instruction: "按任务范围执行", Approval: &workflow.Approval{TimeoutSeconds: 60}}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, ns, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.Repeat("test-secret-", 4)
	t.Setenv("PANGU_HOOK_SECRET_TEST", secret)
	raw, _ := json.Marshal(webhook.Config{ProjectID: project, VersionID: version.ID, Repository: "owner/repo", Label: "pangu", SecretRef: "PANGU_HOOK_SECRET_TEST", Enabled: true})
	w := callAuthorized(h, "POST", "/github/hooks", string(raw), token)
	var hook webhook.Hook
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &hook) != nil {
		t.Fatalf("创建入口 %d %s", w.Code, w.Body.String())
	}
	body := `{"action":"labeled","repository":{"full_name":"owner/repo"},"label":{"name":"pangu"},"issue":{"number":7,"title":"实现任务","body":"验收标准","html_url":"https://github.com/owner/repo/issues/7"}}`
	event := uuid.NewString()
	if w = githubCall(h, hook.ID, event, body, "wrong"); w.Code != 401 {
		t.Fatalf("错误签名 %d", w.Code)
	}
	w = githubCall(h, hook.ID, event, body, secret)
	var receipt submission.Record
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil {
		t.Fatalf("接受 %d %s", w.Code, w.Body.String())
	}
	alias := uuid.NewString()
	for _, delivery := range []string{event, alias} {
		w = githubCall(h, hook.ID, delivery, body, secret)
		if w.Code != 202 || !strings.Contains(w.Body.String(), receipt.ID) {
			t.Fatalf("重放产生重复工作 %d %s", w.Code, w.Body.String())
		}
	}
	if w = githubCall(h, hook.ID, alias, strings.ReplaceAll(body, "验收标准", "其他内容"), secret); w.Code != 409 {
		t.Fatalf("同一别名事件被接受为新任务 %d %s", w.Code, w.Body.String())
	}
	if w = githubCall(h, hook.ID, event, strings.ReplaceAll(body, "验收标准", "其他内容"), secret); w.Code != 409 {
		t.Fatalf("同事件变更内容 %d %s", w.Code, w.Body.String())
	}
	if _, err = conn.Exec(ctx, "UPDATE pangu.memberships SET enabled=false WHERE project_id=$1", project); err != nil {
		t.Fatal(err)
	}
	if w = githubCall(h, hook.ID, uuid.NewString(), body, secret); w.Code != 403 {
		t.Fatalf("撤权后触发 %d %s", w.Code, w.Body.String())
	}
	var count int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM pangu.submissions WHERE namespace=$1", ns).Scan(&count); err != nil || count != 1 {
		t.Fatalf("重复派发 %d %v", count, err)
	}
}
