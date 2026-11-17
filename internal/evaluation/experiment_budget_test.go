package evaluation

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperimentDBConcurrentReservationsConsumeSharedPool(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	renamed := spec
	renamed.ID, renamed.Version = uuid.NewString(), 2
	other, err := CreateExperimentSuite(context.Background(), ns, renamed)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, suite := range []*FrozenSuite{s, other} {
		group.Go(func() {
			_, reserveErr := ReserveExperiment(context.Background(), ns, dbExperimentSpec(suite))
			results <- reserveErr
		})
	}
	group.Wait()
	close(results)
	success, exhausted := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrHoldoutBudget) {
			exhausted++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || exhausted != 1 {
		t.Fatalf("budget race: success=%d exhausted=%d", success, exhausted)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.evaluation_holdout_pools SET used=0 WHERE id=(SELECT pool_id FROM pangu.evaluation_suites WHERE id=$1)", s.ID); err == nil {
		t.Fatal("spent exposure was reset")
	}
	renamed.ID, renamed.Rules.MaxCandidates = uuid.NewString(), 100
	if _, err = CreateExperimentSuite(context.Background(), ns, renamed); err == nil {
		t.Fatal("renamed policy replenished pool")
	}
}

func TestExperimentDBReceiptsAreIdempotentAndRejectOldGeneration(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	x := dbExperimentSpec(s)
	e, err := ReserveExperiment(context.Background(), ns, x)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ReserveExperiment(context.Background(), ns, x)
	if err != nil || again.Hash != e.Hash {
		t.Fatalf("reservation retry consumed twice: %v", err)
	}
	r := pairedReceipts(t, s, e)[0]
	if err = RecordExperimentReceipt(context.Background(), ns, r); err != nil {
		t.Fatal(err)
	}
	if err = RecordExperimentReceipt(context.Background(), ns, r); err != nil {
		t.Fatalf("receipt retry: %v", err)
	}
	r.ProtectionPassed = false
	if err = RecordExperimentReceipt(context.Background(), ns, r); !errors.Is(err, ErrConflict) {
		t.Fatalf("result rewritten: %v", err)
	}
	if err = InvalidateExperiment(context.Background(), ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	r.ProtectionPassed = true
	if err = RecordExperimentReceipt(context.Background(), ns, r); !errors.Is(err, ErrConflict) {
		t.Fatalf("late receipt restored old generation: %v", err)
	}
	x.ID, x.PolicyRevision = uuid.NewString(), 2
	if _, err = ReserveExperiment(context.Background(), ns, x); !errors.Is(err, ErrHoldoutBudget) {
		t.Fatalf("failed experiment refunded exposure: %v", err)
	}
}

func TestExperimentDBRenamingHiddenInputCannotExposeItAsDevelopment(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	if _, err := CreateExperimentSuite(context.Background(), ns, spec); err != nil {
		t.Fatal(err)
	}
	renamed := spec
	renamed.ID = uuid.NewString()
	renamed.Cases = append([]SuiteCase(nil), spec.Cases...)
	renamed.Cases[0].Partition, renamed.Cases[2].Partition = "development", "holdout"
	if _, err := CreateExperimentSuite(context.Background(), ns, renamed); err == nil {
		t.Fatal("hidden cases relabeled as development")
	}
}
