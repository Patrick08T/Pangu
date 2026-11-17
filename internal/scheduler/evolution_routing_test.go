package scheduler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// Sending a private case through the business transport would expose its frozen body.
func TestSubmissionRoutingKeepsPrivateCaseOutOfBusinessTransport(t *testing.T) {
	item := &submission.Record{ID: "request", Operation: "evolution", Snapshot: submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{Input: "private case"}}}
	private, business := 0, 0
	run, err := routeSubmission(context.Background(), "routing", item, func(ctx context.Context, got *submission.Record) (string, error) {
		private++
		if got != item {
			t.Fatal("routing replaced accepted case")
		}
		return "private-run", nil
	}, func(context.Context, *submission.Record) (string, error) {
		business++
		return "business-run", nil
	})
	if err != nil || run != "private-run" || private != 1 || business != 0 {
		t.Fatalf("private case escaped dedicated dispatch: run=%q private=%d business=%d err=%v", run, private, business, err)
	}
}

func TestSubmissionRoutingRejectsMixedPrivateIdentity(t *testing.T) {
	for _, tc := range []struct{ operation, purpose string }{{"evolution", "business"}, {"evolution", ""}, {"task", "evaluation"}} {
		t.Run(tc.operation+"/"+tc.purpose, func(t *testing.T) {
			calls := 0
			send := func(context.Context, *submission.Record) (string, error) { calls++; return "sent", nil }
			item := &submission.Record{Operation: tc.operation, Snapshot: submission.Snapshot{Purpose: tc.purpose}}
			if run, err := routeSubmission(context.Background(), "routing", item, send, send); err == nil || run != "" || calls != 0 {
				t.Fatalf("mixed private identity dispatched: run=%q calls=%d err=%v", run, calls, err)
			}
		})
	}
}

func TestSubmissionRoutingRechecksCatalogBeforeBusinessTransport(t *testing.T) {
	f, item, version := acceptedAutomaticFlow(t)
	before := item.Snapshot
	calls := 0
	private := func(context.Context, *submission.Record) (string, error) {
		t.Fatal("business routed privately")
		return "", nil
	}
	business := func(_ context.Context, got *submission.Record) (string, error) {
		calls++
		if got != item || !reflect.DeepEqual(before, got.Snapshot) {
			t.Fatal("accepted snapshot changed")
		}
		return "business-run", nil
	}
	if run, err := routeSubmission(f.ctx, f.actor.Namespace, item, private, business); err != nil || run != "business-run" || calls != 1 {
		t.Fatalf("valid accepted observing entry rejected: %q %d %v", run, calls, err)
	}
	entryExec(t, `UPDATE pangu.catalog_version_state SET state='revoked',revision=revision+1 WHERE version_id=$1`, version)
	if run, err := routeSubmission(f.ctx, f.actor.Namespace, item, private, business); err == nil || run != "" || calls != 1 {
		t.Fatalf("revoked catalog reached external business lookup/send: %q %d %v", run, calls, err)
	}
}

func TestSubmissionRoutingPreservesPrivateDispatchFailure(t *testing.T) {
	want := errors.New("private proof unavailable")
	item := &submission.Record{Operation: "evolution", Snapshot: submission.Snapshot{Purpose: "evaluation"}}
	private := func(context.Context, *submission.Record) (string, error) { return "", want }
	business := func(context.Context, *submission.Record) (string, error) { return "unsafe fallback", nil }
	if run, err := routeSubmission(context.Background(), "routing", item, private, business); !errors.Is(err, want) || run != "" {
		t.Fatalf("failed private proof fell back to business: %q %v", run, err)
	}
}
