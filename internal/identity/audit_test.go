package identity

import (
	"github.com/Patrick08T/Pangu/internal/audit"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUnavailableAuditBlocksMutationButAllowsCancellation(t *testing.T) {
	t.Setenv("PANGU_DATABASE_URL", "invalid://audit-test")
	spool := t.TempDir()
	t.Setenv("PANGU_AUDIT_SPOOL_ROOT", spool)
	for _, action := range []string{"POST /catalog/{id}/publish", "POST /runs/{id}/cancel"} {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(202) })
		r := httptest.NewRequest("POST", "/runs/"+uuid.NewString()+"/cancel", nil)
		r.Pattern = action
		w := httptest.NewRecorder()
		audited(next, w, r, Actor{ID: uuid.NewString(), Namespace: "test"})
		if called != (action == "POST /runs/{id}/cancel") {
			t.Fatalf("审计门禁错误: %s called=%v", action, called)
		}
		if !called && w.Code != 503 {
			t.Fatal(w.Code)
		}
	}
	files, err := filepath.Glob(filepath.Join(spool, "*", "*.json"))
	if err != nil || len(files) != 2 {
		t.Fatalf("数据库故障后取消缺少持久审计意图和结果: %v %v", files, err)
	}
	for _, file := range files {
		if data, err := os.ReadFile(file); err != nil || len(data) == 0 {
			t.Fatal("审计补录文件为空")
		}
	}
}

func TestAuditResponseExtractsOnlyIdentifiers(t *testing.T) {
	id := uuid.NewString()
	entry := &audit.Entry{}
	w := &auditWriter{ResponseWriter: httptest.NewRecorder(), status: 200}
	_, _ = w.Write([]byte(`{"id":"` + id + `","value":"private-token","spec":{"instruction":"private-content"},"number":3}`))
	w.result(entry)
	if entry.ResultID != id || entry.ResultVersion == nil || *entry.ResultVersion != 3 {
		t.Fatalf("缺少可信结果关联: %+v", entry)
	}
	entry = &audit.Entry{}
	w.status = 403
	w.result(entry)
	if entry.ResultID != "" {
		t.Fatal("失败响应记录了结果")
	}
}
