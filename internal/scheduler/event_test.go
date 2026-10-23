package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

func TestTaskEventsPageBoundsAndRedaction(t *testing.T) {
	task := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stable/tasks/"+task.String()+"/task-events" || r.URL.Query().Get("offset") != "5" || r.URL.Query().Get("limit") != "3" {
			t.Errorf("错误的事件分页请求: %s", r.URL)
		}
		rows := []rest.V1TaskEvent{{Id: 6, TaskId: task, Timestamp: time.Now(), Message: "token=secret-value"}, {Id: 7, TaskId: task, Timestamp: time.Now(), Message: "重试"}, {Id: 8, TaskId: task, Timestamp: time.Now(), Message: "结束"}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rest.V1TaskEventList{Rows: &rows})
	}))
	defer server.Close()
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server.URL)
	t.Setenv("HATCHET_CLIENT_TOKEN", "test-token")
	t.Setenv("HATCHET_CLIENT_TENANT_ID", uuid.NewString())
	page, err := taskEvents(context.Background(), task, 5, 2)
	if err != nil || len(page.Items) != 2 || page.NextOffset == nil || *page.NextOffset != 7 {
		t.Fatalf("分页错误: %+v %v", page, err)
	}
	data, _ := json.Marshal(page)
	if strings.Contains(string(data), "secret-value") {
		t.Fatal("事件泄露凭据")
	}
}
