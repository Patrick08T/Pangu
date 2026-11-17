package evaluation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/identity"
)

// Removing the namespace fence allows registration to win while a source grant
// holds the same lock, leaving its protected-source query stale.
func TestExperimentDBSuiteWaitsForLearningSourceFence(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = identity.Lock(ctx, tx, ns); err != nil {
		t.Fatal(err)
	}
	blocked, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err = CreateExperimentSuite(blocked, ns, spec); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hidden allocation did not wait for learning source fence: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = CreateExperimentSuite(ctx, ns, spec); err != nil {
		t.Fatalf("suite failed after source fence release: %v", err)
	}
}
