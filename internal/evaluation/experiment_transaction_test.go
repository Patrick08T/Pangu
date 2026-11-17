package evaluation

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
)

func TestExperimentDBCallerTransactionFencesSourceAndBudget(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
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
	x := dbExperimentSpec(s)
	nested, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperimentTx(ctx, nested, ns, x)
	if err != nil {
		t.Fatal(err)
	}
	if err = nested.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	read, err := ReadExperimentTx(ctx, tx, ns, s.ProjectID, e.ID)
	if err != nil || read.Hash != e.Hash {
		t.Fatalf("caller tx cannot read reservation: %+v %v", read, err)
	}
	if _, err = LoadExperimentCaseTx(ctx, tx, ns, e.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("uncommitted exposure could be rolled back after reading hidden body")
	}
	r := pairedReceipts(t, s, e)[0]
	if err = RecordExperimentReceiptTx(ctx, tx, ns, r); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.evaluation_cases SET criteria='[\"changed\"]'::jsonb WHERE id=$1", spec.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExperimentTx(ctx, tx, ns, s.ProjectID, e.ID); err == nil {
		t.Fatal("source mutation visible in same tx not fenced")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExperiment(ctx, ns, s.ProjectID, e.ID); err == nil {
		t.Fatal("rolled-back reservation leaked")
	}
	if _, err = ReserveExperiment(ctx, ns, x); err != nil {
		t.Fatalf("rolled-back exposure consumed budget: %v", err)
	}
}

func TestExperimentDBReadAndDecisionStayInCallerTransaction(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(ctx, ns, dbExperimentSpec(s))
	if err != nil {
		t.Fatal(err)
	}
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
	if _, err = ReadExperimentTx(ctx, tx, ns, s.ProjectID, e.ID); err != nil {
		t.Fatal(err)
	}
	read, err := ReadExperimentSuiteTx(ctx, tx, ns, s.ID)
	if err != nil || read.Hash != s.Hash {
		t.Fatalf("suite snapshot mismatch: %+v %v", read, err)
	}
	c, err := LoadExperimentCaseTx(ctx, tx, ns, e.ID, spec.Cases[0].ID)
	if err != nil || c.Input == "" || c.Expected == "" {
		t.Fatalf("trusted case missing: %+v %v", c, err)
	}
	for _, r := range pairedReceipts(t, s, e) {
		if err = RecordExperimentReceiptTx(ctx, tx, ns, r); err != nil {
			t.Fatal(err)
		}
	}
	d, err := EvaluateStoredExperimentTx(ctx, tx, ns, s.ProjectID, e.ID, e.ExperimentSpec)
	if err != nil || d.Category != "primary_threshold" || !validHash(d.DecisionHash) {
		t.Fatalf("uncommitted pairs not evaluated: %+v %v", d, err)
	}
	if _, err = tx.Exec(ctx, "UPDATE pangu.evaluation_cases SET criteria='[\"changed\"]'::jsonb WHERE id=$1", spec.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadExperimentCaseTx(ctx, tx, ns, e.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("case source change ignored")
	}
	if _, err = EvaluateStoredExperimentTx(ctx, tx, ns, s.ProjectID, e.ID, e.ExperimentSpec); err == nil {
		t.Fatal("decision source change ignored")
	}
}
