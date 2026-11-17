package evaluation

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperimentDBTrustedCaseReadRequiresActiveReservation(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	x := dbExperimentSpec(s)
	if _, err = LoadExperimentCase(context.Background(), ns, x.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("unreserved experiment disclosed hidden input")
	}
	e, err := ReserveExperiment(context.Background(), ns, x)
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadExperimentCase(context.Background(), ns, e.ID, spec.Cases[0].ID)
	if err != nil || c.Input == "" || c.Expected == "" {
		t.Fatalf("trusted input missing: %+v %v", c, err)
	}
	if _, err = ReadExperiment(context.Background(), ns, uuid.NewString(), e.ID); err == nil {
		t.Fatal("cross-project experiment read")
	}
	if _, err = LoadExperimentCase(context.Background(), "other-scope", e.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("cross-namespace case read")
	}
	if err = InvalidateExperiment(context.Background(), ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadExperimentCase(context.Background(), ns, e.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("invalid generation disclosed case")
	}
}

func TestExperimentDBChangedSourceBlocksReadAndReceipt(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(context.Background(), ns, dbExperimentSpec(s))
	if err != nil {
		t.Fatal(err)
	}
	r := pairedReceipts(t, s, e)[0]
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.evaluation_cases SET criteria='[\"changed rule\"]'::jsonb WHERE id=$1", spec.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadExperimentCase(context.Background(), ns, e.ID, spec.Cases[0].ID); err == nil {
		t.Fatal("changed trusted assertion accepted")
	}
	if err = RecordExperimentReceipt(context.Background(), ns, r); err == nil {
		t.Fatal("revoked checker emitted valid receipt")
	}
}
