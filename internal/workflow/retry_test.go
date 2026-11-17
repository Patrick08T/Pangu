package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestModelRetryBackoffDoesNotStopAtOrdinaryRetryLimit(t *testing.T) {
	for sequence, seconds := range map[int]int{0: 30, 1: 60, 2: 120, 3: 240, 4: 300, 5: 300, 100000: 300} {
		if got := ModelRetryDelay(sequence); got != time.Duration(seconds)*time.Second {
			t.Fatalf("第 %d 次等待错误: %v", sequence, got)
		}
	}
	for _, kind := range []string{"authentication", "parameter", "output", "execution", "trace", "usage", "budget", "permission", "timeout", "cancelled"} {
		if ModelRetryCategory(kind) {
			t.Fatal("非模型故障被自动重试", kind)
		}
	}
	wrapped := &Failure{Kind: "model_network", Retry: true, Cause: context.DeadlineExceeded}
	if FailureKind(wrapped) != "model_network" {
		t.Fatal("模型网络超时丢失来源")
	}
	if FailureKind(errors.Join(wrapped, context.Canceled)) != "cancelled" {
		t.Fatal("取消未优先于模型故障")
	}
}
