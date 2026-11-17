package scheduler

import (
	"testing"
	"time"
)

// Exercise the actual SDK registration request, not a source constant. The
// isolated transport fixture has dummy credentials and never starts a worker.
func TestPerforceRegistrationAllowsFullLocalCapture(t *testing.T) {
	backend := deadlineBackend(t)
	request, _, _, _ := newPerforceTask(backend.client).Dump()
	if len(request.Tasks) != 1 {
		t.Fatalf("unexpected task registration: %d", len(request.Tasks))
	}
	task := request.Tasks[0]
	budget, err := time.ParseDuration(task.Timeout)
	if err != nil || budget < 2*time.Hour {
		t.Fatalf("engine would cancel an approved local capture: timeout=%q error=%v", task.Timeout, err)
	}
	if task.ScheduleTimeout == nil || *task.ScheduleTimeout != "15s" || task.Retries != 0 {
		t.Fatalf("changed scheduling/retry behavior: %+v", task)
	}
}
