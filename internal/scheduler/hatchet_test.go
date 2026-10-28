package scheduler

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeleteCronAcceptsNoContent(t *testing.T) {
	const id = "4af3fd6f-56cb-4dcd-8f24-269d812c649f"
	const tenant = "417a9d8d-a658-41b2-8f2f-b4a4d9491863"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("invalid delete request")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("HATCHET_CLIENT_SERVER_URL", server.URL)
	t.Setenv("HATCHET_CLIENT_TOKEN", "test-token")
	t.Setenv("HATCHET_CLIENT_TENANT_ID", tenant)
	if err := (Backend{}).deleteHatchetCron(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}

func TestRejectOversizedNodePayload(t *testing.T) {
	input := workflow.NodeInput{Input: strings.Repeat("<", 200*1024)}
	if err := validateNodeInput(input); err == nil {
		t.Fatal("accepted escaped payload over 1 MiB")
	}
}
