package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	legacy "github.com/hatchet-dev/hatchet/pkg/client"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type idempotencyInput struct {
	RequestID string `json:"request_id"`
}

func TestHatchetIdempotencySurvivesLostAcknowledgement(t *testing.T) {
	_, client := integrationClient(t)
	var executions atomic.Int32
	task := client.NewStandaloneTask("pangu-idempotency-probe", func(ctx hatchet.Context, input idempotencyInput) (idempotencyInput, error) {
		executions.Add(1)
		return input, nil
	}, hatchet.WithWorkflowIdempotency(hatchet.IdempotencyConfig{Expression: "input.request_id", TTL: 90 * 24 * time.Hour, Method: hatchet.IdempotencyMethodTTL}))
	worker, err := client.NewWorker("idempotency-probe", hatchet.WithWorkflows(task))
	if err != nil {
		t.Fatal(err)
	}
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		if _, err := client.Workflows().Get(ctx, task.GetName()); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	input := idempotencyInput{RequestID: uuid.NewString()}
	ids := make(chan string, 4)
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			ref, err := task.RunNoWait(ctx, input)
			if err == nil {
				ids <- ref.RunId
				return
			}
			var collision *hatchet.IdempotencyCollisionError
			if !errors.As(err, &collision) {
				t.Errorf("非预期提交错误: %T %v", err, err)
				return
			}
			ids <- collision.ExistingRunExternalId
		})
	}
	group.Wait()
	close(ids)
	var accepted string
	for id := range ids {
		if accepted != "" && id != accepted {
			t.Fatalf("同键产生不同运行: %s %s", accepted, id)
		}
		accepted = id
	}
	if accepted == "" {
		t.Fatal("没有接受的运行")
	}
	for executions.Load() == 0 && ctx.Err() == nil {
		time.Sleep(50 * time.Millisecond)
	}
	_, err = client.RunNoWait(ctx, "pangu-idempotency-probe", input)
	var collision *legacy.IdempotencyViolationErr
	if !errors.As(err, &collision) || collision.ExistingRunExternalId != accepted || executions.Load() != 1 {
		t.Fatalf("丢失回执后无法恢复原运行: count=%d err=%T %v", executions.Load(), err, err)
	}
	t.Logf("并发和丢失回执均关联同一运行: %s，执行次数=1", accepted)
}
