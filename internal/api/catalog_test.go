package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/schedule"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/Patrick08T/Pangu/internal/workspace"
	"github.com/google/uuid"
)

func TestArchivedVersionDoesNotPreventPausingExistingCron(t *testing.T) {
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := workspace.Register(ctx, "api-test", workspace.Project{Name: "暂停测试", Source: t.TempDir(), Branch: "main", WorkerID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := catalog.Create(ctx, "api-test", catalog.NewObject{Kind: "agent", ProjectID: p.ID, Name: "固定版本", Spec: catalog.Spec{Agent: &workflow.Node{ID: "one", Executor: "codex", Instruction: "检查"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := catalog.Publish(ctx, "api-test", a.ID, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.Compile(ctx, "api-test", v.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	c := workflow.Cron{Name: "pause-" + uuid.NewString(), Expression: "0 1 * * *", Workflow: *d}
	plan, err := schedule.Create(ctx, "api-test", c)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.Archive(ctx, "api-test", a.ID, true); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"revision": plan.Revision, "paused": true, "cron": c})
	r := httptest.NewRequest("PUT", "/crons/"+plan.ID, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b := &fakeBackend{}
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 200 || b.calls != 1 || b.cron.Workflow.VersionID != v.ID {
		t.Fatalf("归档后无法暂停: %d %s", w.Code, w.Body.String())
	}
}
