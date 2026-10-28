package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicSubmitCannotOccupyInternalDeliveryKey(t *testing.T) {
	b := &fakeBackend{}
	r := httptest.NewRequest("POST", "/runs", strings.NewReader(validRun))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "delivery:"+testID)
	w := httptest.NewRecorder()
	testAPI(b).ServeHTTP(w, r)
	if w.Code != 400 || b.calls != 0 {
		t.Fatalf("公共请求抢占内部交付键 %d %d", w.Code, b.calls)
	}
}
