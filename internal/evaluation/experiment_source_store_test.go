package evaluation

import (
	"context"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperimentDBAuthenticatesSourceTaskAndProject(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	for _, change := range []func(*SuiteSpec){
		func(s *SuiteSpec) { s.Cases[0].TaskID = uuid.NewString() },
		func(s *SuiteSpec) { s.Cases[0].SourceRunID = uuid.NewString() },
		func(s *SuiteSpec) { s.ProjectID = uuid.NewString() },
		func(s *SuiteSpec) { s.Cases[0].InputHash = strings.Repeat("a", 64) },
	} {
		bad := spec
		bad.Cases = append([]SuiteCase(nil), spec.Cases...)
		change(&bad)
		if _, err := CreateExperimentSuite(context.Background(), ns, bad); err == nil {
			t.Fatal("unauthenticated source accepted")
		}
	}
}

func TestExperimentDBPurgedSourceInvalidatesReservedExperiment(t *testing.T) {
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
	if _, err = conn.Exec(ctx, "SELECT pangu.purge_run_payloads($1)", spec.Cases[0].SourceRunID); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExperiment(ctx, ns, s.ProjectID, e.ID); err == nil {
		t.Fatal("source purge left experiment executable")
	}
	if _, err = EvaluateStoredExperiment(ctx, ns, s.ProjectID, e.ID, e.ExperimentSpec); err == nil {
		t.Fatal("purged source produced verdict")
	}
}

func TestExperimentDBCannotReassignKnownInputToNewIndependentFamily(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixture(t)
	if _, err := CreateExperimentSuite(ctx, ns, spec); err != nil {
		t.Fatal(err)
	}
	spec.ID = uuid.NewString()
	spec.Cases[0].FamilyID = "invented-independent-family"
	if _, err := CreateExperimentSuite(ctx, ns, spec); err == nil {
		t.Fatal("input independence reassigned after exposure")
	}
}

func TestExperimentDBPassedDecisionUsesStoredPairs(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixtureN(t, 20)
	s, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(ctx, ns, dbExperimentSpec(s))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range pairedReceipts(t, s, e) {
		if err = RecordExperimentReceipt(ctx, ns, r); err != nil {
			t.Fatal(err)
		}
	}
	d, err := EvaluateStoredExperiment(ctx, ns, s.ProjectID, e.ID, e.ExperimentSpec)
	if err != nil || d.Outcome != "passed" || !validHash(d.DecisionHash) {
		t.Fatalf("complete independent pairs not evaluated: %+v %v", d, err)
	}
	if err = InvalidateExperiment(ctx, ns, e.ID, e.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = EvaluateStoredExperiment(ctx, ns, s.ProjectID, e.ID, e.ExperimentSpec); err == nil {
		t.Fatal("old passed generation refreshed verdict")
	}
}
