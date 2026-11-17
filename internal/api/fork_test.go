package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/scheduler"
)

type sourceBodyRemovedBackend struct {
	fakeBackend
	unknown bool
}

func (b *sourceBodyRemovedBackend) GetRun(ctx context.Context, id string) (*scheduler.RunDetails, error) {
	if b.unknown {
		b.readCall()
		return nil, scheduler.ErrNotFound
	}
	item, err := b.fakeBackend.GetRun(ctx, id)
	item.LogsExpired = true
	item.Definition = nil
	return item, err
}

func TestForkReceiptRouteRequiresBusinessProvenanceNotSourceBody(t *testing.T) {
	for _, tc := range []struct {
		name              string
		unknown           bool
		status, mutations int
	}{{"business_tombstone", false, 202, 1}, {"unknown_source", true, 404, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			b := &sourceBodyRemovedBackend{unknown: tc.unknown}
			h := testAPI(b)
			var prior string
			for attempt := 1; attempt <= 2; attempt++ {
				r := httptest.NewRequest("POST", "/runs/"+testID+"/fork", strings.NewReader(`{"checkpoint_node":"writer","workflow":`+validRun+`}`))
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Idempotency-Key", "fork-receipt")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.status || b.mutations != tc.mutations*attempt {
					t.Fatalf("fork provenance: %d mutations=%d %s", w.Code, b.mutations, w.Body.String())
				}
				if !tc.unknown && (w.Header().Get("Location") != "/api/requests/"+testID || !strings.Contains(w.Body.String(), testID)) {
					t.Fatalf("missing existing receipt: %s", w.Body.String())
				}
				if attempt == 2 && w.Body.String() != prior {
					t.Fatal("retry changed the route's receipt")
				}
				prior = w.Body.String()
			}
		})
	}
}
