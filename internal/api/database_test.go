package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessReportsDependenciesWithoutSecrets(t *testing.T) {
	for _, failed := range []bool{false, true} {
		b := &fakeBackend{}
		h := handler{backend: b, checkDatabase: func(context.Context) error {
			if failed {
				return errors.New("private-secret")
			}
			return nil
		}}
		w := httptest.NewRecorder()
		h.ready(w, httptest.NewRequest("GET", "/readyz", nil))
		expected := 200
		if failed {
			expected = 503
		}
		if w.Code != expected || strings.Contains(w.Body.String(), "private-secret") || !strings.Contains(w.Body.String(), "api_dependencies") {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	}
}
