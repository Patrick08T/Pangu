package scheduler

import (
	"testing"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
)

func TestWorkerNeedsFreshHeartbeatAndBothActions(t *testing.T) {
	now, status := time.Now(), rest.ACTIVE
	actions := []string{"project_" + WorkflowName + ":" + WorkflowName, "project_" + PreflightTaskName + ":" + PreflightTaskName}
	worker := rest.Worker{Status: &status, LastHeartbeatAt: &now, Actions: &actions}
	if !workerSupports(worker, "project_"+WorkflowName, now) {
		t.Fatal("在线且具有所需能力的 Worker 被拒绝")
	}
	if workerSupports(worker, "other_"+WorkflowName, now) || workerSupports(worker, "project_"+WorkflowName, now.Add(time.Minute)) {
		t.Fatal("接受其他命名空间或过期心跳")
	}
	status = rest.PAUSED
	if workerSupports(worker, "project_"+WorkflowName, now) {
		t.Fatal("暂停的 Worker 不应接受新提交")
	}
}
