package evaluation

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func manualCaseRequest(t *testing.T, ns, project, input string) (string, string) {
	t.Helper()
	ctx := context.Background()
	handoff := &workflow.HandoffContract{Objective: "验证", Inputs: "输入", Deliverables: "结果", Acceptance: "独立检查", FailureHandling: "报告失败"}
	object, err := catalog.Create(ctx, ns, catalog.NewObject{Kind: "agent", Name: "评测助手", Spec: catalog.Spec{Agent: &workflow.Node{ID: "agent", Executor: "codex", Instruction: "独立检查", Handoff: handoff}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalog.Publish(ctx, ns, object.ID, object.Revision)
	if err != nil {
		t.Fatal(err)
	}
	request, _, _ := databaseSubmissionFixture(t, ns, workflow.Definition{ProjectID: project, VersionID: version.ID, Input: input}, "", false)
	return version.ID, request
}

func TestExperimentDBHiddenCasesCannotLeakThroughManualRuns(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixture(t)
	c, err := ReadCase(ctx, ns, spec.Cases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	version, request := manualCaseRequest(t, ns, spec.ProjectID, c.Input)
	if err = LinkRun(ctx, ns, c.ID, version, request); err != nil {
		t.Fatal(err)
	}
	before, err := CaseRuns(ctx, ns, c.ID, 0)
	if err != nil || len(before) != 1 {
		t.Fatalf("fixture missing link: %+v %v", before, err)
	}
	if _, err = CreateExperimentSuite(ctx, ns, spec); err != nil {
		t.Fatal(err)
	}
	after, err := CaseRuns(ctx, ns, c.ID, 0)
	if err != nil || len(after) != 0 {
		t.Errorf("hidden source feedback leaked: %+v %v", after, err)
	}
	if err = LinkRun(ctx, ns, c.ID, version, request); !errors.Is(err, ErrConflict) {
		t.Errorf("hidden case accepted manual path: %v", err)
	}
}

func TestExperimentDBDuplicateHiddenInputIsMaskedOnCreation(t *testing.T) {
	ctx := context.Background()
	ns, spec := dbSuiteFixture(t)
	c, err := ReadCase(ctx, ns, spec.Cases[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CreateExperimentSuite(ctx, ns, spec); err != nil {
		t.Fatal(err)
	}
	duplicate, err := CreateCase(ctx, ns, spec.ProjectID, c.ActorID, CaseInput{Name: "cloned", Input: c.Input, Expected: c.Expected, Criteria: c.Criteria, RetainInput: true})
	if err != nil || duplicate.Name != "" || duplicate.Input != "" || duplicate.Expected != "" || len(duplicate.Criteria) != 0 {
		t.Fatalf("duplicate disclosed hidden body: %+v %v", duplicate, err)
	}
}
