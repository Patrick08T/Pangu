//go:build integration

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func evolutionStartHash(value any) string {
	raw, _ := json.Marshal(value)
	return evolutionStartTextHash(string(raw))
}

func evolutionStartTextHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (f evolutionStartAPIFixture) createSuite(t *testing.T) *evaluation.FrozenSuite {
	t.Helper()
	ctx := context.Background()
	spec := evaluation.SuiteSpec{ID: uuid.NewString(), ProjectID: f.project, Version: 1, Rules: evaluation.DecisionRules{Primary: "acceptance", Method: "hoeffding_bonferroni", FailurePolicy: "invalidate", Confidence: .95, MinFamilies: 1, Repeats: 1, MaxCandidates: 2, MinImprovement: .1, Metrics: []evaluation.MetricRule{{Name: "acceptance", HigherBetter: true, Lower: 0, Upper: 1}, {Name: "cost", Lower: 0, Upper: 100, Tolerance: 2}, {Name: "duration", Lower: 0, Upper: 100, Tolerance: 2}}}}
	for i, partition := range []string{"holdout", "protection", "development"} {
		input, expected := fmt.Sprintf("private case %d", i), `{"answer":"checker-only-value"}`
		c, err := evaluation.CreateCase(ctx, f.namespace, f.project, f.actor, evaluation.CaseInput{Name: "case", Input: input, Expected: expected, Criteria: []string{"json_equal"}, RetainInput: true})
		if err != nil {
			t.Fatal(err)
		}
		s, err := submission.Prepare(ctx, f.namespace, "source", uuid.NewString(), submission.Snapshot{Purpose: "evaluation", Workflow: workflow.Definition{ProjectID: f.project, Input: input}})
		if err != nil {
			t.Fatal(err)
		}
		run := uuid.NewString()
		if _, err = submission.Dispatch(ctx, f.namespace, s.ID, func(context.Context, *submission.Record) (string, error) { return run, nil }); err != nil {
			t.Fatal(err)
		}
		spec.Cases = append(spec.Cases, evaluation.SuiteCase{ID: c.ID, FamilyID: uuid.NewString(), TaskID: s.TaskID, SourceRunID: run, InputHash: evolutionStartTextHash(input), ExpectedHash: evolutionStartTextHash(expected), CriteriaHash: evolutionStartHash(c.Criteria), CheckerHash: evolution.CheckerHash(), Partition: partition, Critical: partition == "protection"})
	}
	suite, err := evaluation.CreateExperimentSuite(ctx, f.namespace, spec)
	if err != nil {
		t.Fatal(err)
	}
	return suite
}
