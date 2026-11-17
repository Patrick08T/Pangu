package evaluation

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

func TestExperimentDBPriorLearningExposureCannotBecomeHidden(t *testing.T) {
	for _, withdrawn := range []bool{false, true} {
		t.Run(map[bool]string{false: "granted", true: "withdrawn"}[withdrawn], func(t *testing.T) {
			ns, spec := dbSuiteFixture(t)
			exposureFixture(t, ns, spec, withdrawn, false)
			if _, err := CreateExperimentSuite(context.Background(), ns, spec); !errors.Is(err, ErrInvalid) {
				t.Fatalf("previous learning exposure became hidden: %v", err)
			}
			spec.ID = uuid.NewString()
			if _, err := CreateExperimentSuite(context.Background(), ns, spec); !errors.Is(err, ErrInvalid) {
				t.Fatalf("renamed suite erased exposure: %v", err)
			}
		})
	}
}

func TestExperimentDBCopiedInputCannotEscapeLearningExposure(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	exposureFixture(t, ns, spec, false, false)
	ctx := context.Background()
	original, err := ReadCase(ctx, ns, spec.Cases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := CreateCase(ctx, ns, spec.ProjectID, original.ActorID, CaseInput{Name: "unrelated-looking copy", Input: original.Input, Expected: original.Expected, Criteria: original.Criteria, RetainInput: true})
	if err != nil {
		t.Fatal(err)
	}
	spec.Cases[0].ID = copied.ID
	spec.Cases[0].SourceRunID, spec.Cases[0].TaskID = sourceRunFixture(t, ns, spec.ProjectID, original.Input)
	if _, err = CreateExperimentSuite(ctx, ns, spec); !errors.Is(err, ErrInvalid) {
		t.Fatalf("same input with new case/task/run escaped exposure: %v", err)
	}
}

func TestExperimentDBUnresolvedExposureFailsClosedForProject(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	exposureFixture(t, ns, spec, true, true)
	if _, err := CreateExperimentSuite(context.Background(), ns, spec); !errors.Is(err, ErrInvalid) {
		t.Fatalf("purged historical exposure treated as clean: %v", err)
	}
	other, independent := dbSuiteFixture(t)
	if _, err := CreateExperimentSuite(context.Background(), other, independent); err != nil {
		t.Fatalf("unknown history crossed project scope: %v", err)
	}
}

func TestExperimentDBBackfilledExposureInvalidatesExistingSuite(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	ctx := context.Background()
	s, err := CreateExperimentSuite(ctx, ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReserveExperiment(ctx, ns, dbExperimentSpec(s))
	if err != nil {
		t.Fatal(err)
	}
	exposureFixture(t, ns, spec, true, false)
	if _, err = ReadExperimentSuite(ctx, ns, s.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("suite ignored historical exposure: %v", err)
	}
	if _, err = ReadExperiment(ctx, ns, s.ProjectID, e.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("experiment ignored historical exposure: %v", err)
	}
	if _, err = LoadExperimentCase(ctx, ns, e.ID, spec.Cases[0].ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hidden body exposed after history backfill: %v", err)
	}
	if err = RecordExperimentReceipt(ctx, ns, pairedReceipts(t, s, e)[0]); !errors.Is(err, ErrInvalid) {
		t.Fatalf("receipt ignored exposure: %v", err)
	}
	if _, err = EvaluateStoredExperiment(ctx, ns, s.ProjectID, e.ID, e.ExperimentSpec); !errors.Is(err, ErrInvalid) {
		t.Fatalf("decision ignored exposure: %v", err)
	}
}

func TestExperimentDBExposureDoesNotForbidDevelopmentOnlyUse(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	dev := spec
	dev.Cases = append([]SuiteCase(nil), spec.Cases...)
	for _, c := range spec.Cases {
		if c.Partition == "development" {
			dev.Cases[0] = c
			break
		}
	}
	id := exposureFixture(t, ns, dev, true, false)
	if _, err := CreateExperimentSuite(context.Background(), ns, spec); err != nil {
		t.Fatalf("development exposure wrongly forbids hidden unrelated cases: %v", err)
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), `DELETE FROM pangu.learning_source_exposures WHERE consent_id=$1`, id); err == nil {
		t.Fatal("exposure tombstone can be erased")
	}
}
