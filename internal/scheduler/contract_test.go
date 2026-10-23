package scheduler

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

func TestRunStatusNeverInventsCompletion(t *testing.T) {
	cases := map[rest.V1TaskStatus]string{
		"QUEUED": "queued", "RUNNING": "running", "COMPLETED": "succeeded",
		"FAILED": "failed", "CANCELLED": "cancelled", "FUTURE_STATE": "unknown", "": "unknown",
	}
	for input, want := range cases {
		if got := runStatus(input); got != want {
			t.Errorf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestRunQueryBoundsAndSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)
	q, err := ParseRunQuery(url.Values{}, now)
	if err != nil || q.Limit != 20 || !q.Until.Equal(now) || q.Offset != 0 {
		t.Fatalf("default query: %+v %v", q, err)
	}
	for _, raw := range []string{"limit=101", "limit=0", "offset=-1", "status=done", "since=no",
		"since=2026-10-24T00:00:00Z", "until=bad", "typo=1", "limit=1&limit=2"} {
		values, _ := url.ParseQuery(raw)
		if _, err := ParseRunQuery(values, now); err == nil {
			t.Errorf("accepted invalid query %s", raw)
		}
	}
}

func TestRunQueryAcceptsProjectAndCancellingStatus(t *testing.T) {
	now := time.Now()
	q, err := ParseRunQuery(url.Values{"project_id": {"00000000-0000-0000-0000-000000000001"}, "status": {"cancelling"}}, now)
	if err != nil || q.ProjectID == "" || q.Status != "cancelling" {
		t.Fatalf("无法筛选当前项目的取消中运行: %+v %v", q, err)
	}
	if _, err = ParseRunQuery(url.Values{"project_id": {"invalid"}}, now); err == nil {
		t.Fatal("接受无效项目")
	}
}

func TestNodeProjectionExcludesInternalPayloadAndKeepsAttempt(t *testing.T) {
	var row rest.V1TaskSummary
	err := json.Unmarshal([]byte(`{"status":"FAILED","retryCount":2,"input":{"node":{"id":"writer","executor":"eino","instruction":"private"}},"output":{},"errorMessage":"token=private-secret","additionalMetadata":{"pangu_node_id":"writer"}}`), &row)
	if err != nil {
		t.Fatal(err)
	}
	node := nodeRun(row)
	if node.NodeID != "writer" || node.Attempt != 3 || node.Status != "failed" || node.Error == nil {
		t.Fatalf("unexpected projection: %+v", node)
	}
	data, _ := json.Marshal(node)
	var fields map[string]any
	_ = json.Unmarshal(data, &fields)
	for _, key := range []string{"input", "additionalMetadata", "workflowConfig", "instruction"} {
		if _, exists := fields[key]; exists {
			t.Errorf("exposed internal field %s", key)
		}
	}
}

func TestProjectionOmitsHatchetEmptyErrorsAndZeroTimes(t *testing.T) {
	var row rest.V1TaskSummary
	if err := json.Unmarshal([]byte(`{"status":"COMPLETED","errorMessage":"","startedAt":"0001-01-01T00:00:00Z","finishedAt":"0001-01-01T00:00:00Z"}`), &row); err != nil {
		t.Fatal(err)
	}
	node := nodeRun(row)
	if node.Error != nil || node.StartedAt != nil || node.FinishedAt != nil {
		t.Fatalf("空错误或零值时间被展示为真实结果: %+v", node)
	}
}
