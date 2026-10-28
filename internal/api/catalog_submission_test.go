package api

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

type savedVersionBackend struct{ fakeBackend }

func (b *savedVersionBackend) Submit(ctx context.Context, d workflow.Definition, key string) (*submission.Record, error) {
	return submission.Find(ctx, b.Namespace(), "submit", key, submission.Snapshot{Workflow: d})
}

func TestArchivedVersionReturnsExistingReceiptButRejectsNewOrChangedRequest(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := workspace.Register(ctx, "api-test", workspace.Project{Name: "归档回执", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	object, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "agent", Name: "固定版本", Spec: catalog.Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Handoff: testHandoff(), Instruction: "检查"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.Compile(ctx, "api-test", v.ID, p.ID, "原始输入")
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	record, err := submission.Prepare(ctx, "api-test", "submit", key, submission.Snapshot{Workflow: *d})
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.Archive(ctx, "api-test", object.ID, true); err != nil {
		t.Fatal(err)
	}
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `UPDATE pangu.submissions SET snapshot=snapshot #- '{workflow,nodes}' #- '{workflow,input}' WHERE id=$1`, record.ID); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		key, input string
		status     int
	}{{key, "原始输入", 202}, {key, "改写输入", 409}, {uuid.NewString(), "原始输入", 409}} {
		request := httptest.NewRequest("POST", "/versions/"+strings.ToUpper(v.ID)+"/run", strings.NewReader(`{"project_id":"`+p.ID+`","input":"`+item.input+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", item.key)
		response := httptest.NewRecorder()
		New(&savedVersionBackend{}, testAccess{}).ServeHTTP(response, request)
		if response.Code != item.status {
			t.Fatalf("%s %s: %d %s", item.key, item.input, response.Code, response.Body.String())
		}
		if item.status == 202 && !strings.Contains(response.Body.String(), record.ID) {
			t.Fatal("未返回原提交回执")
		}
	}
}
