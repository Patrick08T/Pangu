package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Policy struct {
	Retries int `json:"retries"`
}

func (n Node) EffectivePolicy() Policy {
	if n.Policy != nil {
		return *n.Policy
	}
	return Policy{Retries: 2}
}

func (n Node) validatePolicy() error {
	p := n.EffectivePolicy()
	if p.Retries < 0 || p.Retries > 2 {
		return fmt.Errorf("节点 %s 策略越界：重试 0–2 次", n.ID)
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
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Kind
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "execution"
}

// ModelRetryCategory 只接受提供方适配层确认的临时模型故障。
func ModelRetryCategory(kind string) bool {
	return kind == "model_unavailable" || kind == "model_network" || kind == "rate_limit"
}

// ModelRetryDelay 的序号从 0 开始；持续重试仍受取消和显式调用预算约束。
func ModelRetryDelay(sequence int) time.Duration {
	if sequence < 0 || sequence >= 4 {
		return 5 * time.Minute
	}
	return 30 * time.Second * time.Duration(1<<sequence)
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
