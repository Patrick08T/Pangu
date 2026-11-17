//go:build integration

package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/google/uuid"
)

func TestEvolutionRevocationDuringModelCallStopsFollowingNodes(t *testing.T) {
	var calls atomic.Int32
	var f evolutionFixture
	admin := uuid.NewString()
	revocation := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			// This acquires the production identity lock. It must not wait on a transaction spanning model RPC.
			revocation <- identity.Disable(ctx, identity.Actor{ID: admin, Namespace: f.scope.Namespace}, f.scope.ActorID)
		}
		evolutionResponse(w, true)
	}))
	defer server.Close()
	var pRequest, run string
	f, p, runID := evolutionStartedFixture(t, server.URL)
	pRequest, run = p.RequestID, runID
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$3,true)`, admin, f.scope.Namespace, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err = RunEvolutionCase(ctx, f.scope.Namespace, pRequest, run); err == nil {
		t.Fatal("revoked actor completed flow")
	}
	if err = <-revocation; err != nil {
		t.Fatalf("authorization transaction blocked by provider call: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("revoked execution reached later model node: %d", calls.Load())
	}
}
