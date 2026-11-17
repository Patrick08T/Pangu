package scheduler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

type preflightRoundTrip func(*http.Request) (*http.Response, error)

func (f preflightRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPreflightRegistrationMayTakeLongerThanOldBudget(t *testing.T) {
	b := deadlineBackend(t)
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	http.DefaultTransport = preflightRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"rows":[]}`
		if strings.HasSuffix(r.URL.Path, "/workflows") {
			select {
			case <-time.After(11 * time.Second):
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			body = `{"rows":[{"name":"` + b.Namespace() + `","metadata":{"id":"11111111-1111-4111-8111-111111111111"}}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	result, err := b.Preflight(t.Context(), workflow.Definition{})
	if err != nil || result == nil || result.Ready || len(result.Checks) != 1 || result.Checks[0].Component != "worker" {
		t.Fatalf("slow registration did not reach worker availability check: result=%+v err=%v", result, err)
	}
}

func TestPreflightRegistrationRespectsFiniteAndParentDeadlines(t *testing.T) {
	b := deadlineBackend(t)
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	http.DefaultTransport = preflightRoundTrip(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	for _, parentBudget := range []time.Duration{time.Millisecond, time.Minute} {
		t.Run(parentBudget.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), parentBudget)
			defer cancel()
			started := time.Now()
			result, err := b.Preflight(ctx, workflow.Definition{})
			elapsed := time.Since(started)
			if result != nil || !errors.Is(err, context.DeadlineExceeded) || elapsed > parentBudget+time.Second || elapsed >= 30*time.Second {
				t.Fatalf("registration extended parent or exceeded HTTP budget: result=%+v err=%v elapsed=%s", result, err, elapsed)
			}
		})
	}
}
