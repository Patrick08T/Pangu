package evaluation

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestExperimentDBFamilyCannotCrossPartitionsAcrossSuites(t *testing.T) {
	ctx := context.Background()
	ns, first := dbSuiteFixture(t)
	c, err := ReadCase(ctx, ns, first.Cases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CreateExperimentSuite(ctx, ns, first); err != nil {
		t.Fatal(err)
	}
	second := suiteFixture(1)
	second.ID = uuid.NewString()
	second.ProjectID = first.ProjectID
	second.Rules.MaxCandidates = 1
	for i := range second.Cases {
		input := fmt.Sprintf("new independent input %s %d", second.ID, i)
		saved, err := CreateCase(ctx, ns, first.ProjectID, c.ActorID, CaseInput{Name: "new", Input: input, Expected: "answer", Criteria: []string{"assert"}, RetainInput: true})
		if err != nil {
			t.Fatal(err)
		}
		run, task := sourceRunFixture(t, ns, first.ProjectID, input)
		entry := &second.Cases[i]
		entry.ID = saved.ID
		entry.FamilyID = uuid.NewString()
		entry.TaskID = task
		entry.SourceRunID = run
		entry.InputHash = textHash(input)
		entry.ExpectedHash = textHash(saved.Expected)
		entry.CriteriaHash = contentHash(saved.Criteria)
	}
	second.Cases[2].FamilyID = first.Cases[0].FamilyID
	if _, err = CreateExperimentSuite(ctx, ns, second); err == nil {
		t.Fatal("hidden family leaked across suites through new inputs")
	}
	second.Cases[2].FamilyID = uuid.NewString()
	if _, err = CreateExperimentSuite(ctx, ns, second); err != nil {
		t.Fatalf("truly independent replacement rejected: %v", err)
	}
}
