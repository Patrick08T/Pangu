package evaluation

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func dbSuiteFixture(t *testing.T) (string, SuiteSpec) {
	return dbSuiteFixtureN(t, 1)
}

func dbSuiteFixtureN(t *testing.T, families int) (string, SuiteSpec) {
	t.Helper()
	ns, project, user := experimentDBScope(t)
	spec := suiteFixture(families)
	spec.ID, spec.ProjectID, spec.Rules.MaxCandidates = uuid.NewString(), project, 1
	for i := range spec.Cases {
		input := fmt.Sprintf("independent input %d", i)
		c, err := CreateCase(context.Background(), ns, project, user, CaseInput{Name: input,
			Input: input, Expected: "trusted answer", Criteria: []string{"machine assertion"}, RetainInput: true})
		if err != nil {
			t.Fatal(err)
		}
		run, task := sourceRunFixture(t, ns, project, input)
		spec.Cases[i].ID, spec.Cases[i].TaskID, spec.Cases[i].SourceRunID = c.ID, task, run
		spec.Cases[i].InputHash = fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
		spec.Cases[i].ExpectedHash = fmt.Sprintf("%x", sha256.Sum256([]byte("trusted answer")))
		spec.Cases[i].CriteriaHash = contentHash(c.Criteria)
	}
	return ns, spec
}

func sourceRunFixture(t *testing.T, ns, project, input string) (string, string) {
	t.Helper()
	_, task, run := databaseSubmissionFixture(t, ns, workflow.Definition{ProjectID: project, Input: input}, "", true)
	return run, task
}

func dbExperimentSpec(s *FrozenSuite) ExperimentSpec {
	e := experimentFixture(s)
	e.ID, e.EnvironmentID, e.BaselineVersion = uuid.NewString(), uuid.NewString(), uuid.NewString()
	return e
}

func TestExperimentDBFrozenCasesRemainHiddenAndImmutable(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadCase(context.Background(), ns, spec.Cases[0].ID)
	if err != nil || read.Input != "" || read.Expected != "" || len(read.Criteria) != 0 {
		t.Fatalf("hidden answer leaked: %+v %v", read, err)
	}
	cases, err := Cases(context.Background(), ns, spec.ProjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.ID == spec.Cases[0].ID && c.Input != "" {
			t.Fatal("list leaked holdout")
		}
	}
	conn, err := database.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.evaluation_suites SET spec='{}'::jsonb WHERE id=$1", s.ID); err == nil {
		t.Fatal("suite threshold writable")
	}
	if _, err = conn.Exec(context.Background(), "UPDATE pangu.evaluation_holdout_pools SET used=0 WHERE id=(SELECT pool_id FROM pangu.evaluation_suites WHERE id=$1)", s.ID); err != nil {
		t.Fatal(err)
	}
	bad := spec
	bad.ID = uuid.NewString()
	bad.Cases = append([]SuiteCase(nil), spec.Cases...)
	bad.Cases[0].ExpectedHash = strings.Repeat("0", 64)
	if _, err = CreateExperimentSuite(context.Background(), ns, bad); err == nil {
		t.Fatal("forged retained expected hash accepted")
	}
}

func TestExperimentDBSourceRevocationPreventsReservation(t *testing.T) {
	ns, spec := dbSuiteFixture(t)
	s, err := CreateExperimentSuite(context.Background(), ns, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err = RemoveCase(context.Background(), ns, spec.Cases[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ReserveExperiment(context.Background(), ns, dbExperimentSpec(s)); err == nil {
		t.Fatal("revoked source allowed evaluation")
	}
}
