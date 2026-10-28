package workflow

import (
	"context"
	"errors"
	"fmt"
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
