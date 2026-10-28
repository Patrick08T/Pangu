package workflow

import (
	"context"
	"errors"
	"fmt"
)

type Policy struct {
	Retries        int `json:"retries"`
	TimeoutSeconds int `json:"timeout_seconds"`
	ToolCalls      int `json:"tool_calls"`
}

func (n Node) EffectivePolicy() Policy {
	if n.Policy != nil {
		return *n.Policy
	}
	seconds := 180
	if n.Executor == "codex" {
		seconds = 1800
	}
	return Policy{Retries: 2, TimeoutSeconds: seconds, ToolCalls: 10}
}

func (n Node) validatePolicy() error {
	p := n.EffectivePolicy()
	maxSeconds := 180
	if n.Executor == "codex" {
		maxSeconds = 1800
	}
	if p.Retries < 0 || p.Retries > 2 || p.TimeoutSeconds < 1 || p.TimeoutSeconds > maxSeconds || p.ToolCalls < 0 || p.ToolCalls > 10 {
		return fmt.Errorf("节点 %s 策略越界：重试 0–2 次，超时 1–%d 秒，工具 0–10 次", n.ID, maxSeconds)
	}
	return nil
}

type Failure struct {
	Kind  string
	Cause error
	Retry bool
}

func (f *Failure) Error() string { return fmt.Sprintf("[%s] %v", f.Kind, f.Cause) }
func (f *Failure) Unwrap() error { return f.Cause }

func FailureKind(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Kind
	}
	return "execution"
}

func CanRetry(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrStopUnconfirmed) {
		return false
	}
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Retry
	}
	return true
}
