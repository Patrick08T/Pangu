package scheduler

import (
	"testing"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func evolutionPlanFixture() evolution.ExecutionPlan {
	p := evolution.ExecutionPlan{RequestID: uuid.NewString(), Key: evolution.CaseKey{IntentID: uuid.NewString(), Generation: 1, Arm: "candidate", CaseID: uuid.NewString(), Repeat: 1}, MaxCalls: 4, MaxTokens: 1000}
	p.Environment = evolution.Environment{ID: uuid.NewString(), ProjectID: uuid.NewString(), Backend: "eino_json_v1", Enabled: true, ConnectionVersion: uuid.NewString(), Currency: "USD", Hash: "environment"}
	p.Definition = workflow.Definition{ProjectID: p.Environment.ProjectID, Input: "case input only", Nodes: []workflow.Node{{ID: "answer", Executor: "eino", Model: "fixture", Instruction: "return JSON", ConnectionVersion: p.Environment.ConnectionVersion}}}
	p.Experiment = evaluation.FrozenExperiment{}
	p.Experiment.LogicalProjectID = p.Definition.ProjectID
	p.Experiment.EnvironmentHash = p.Environment.Hash
	p.Experiment.Generation = 1
	p.Experiment.Purpose = "evaluation"
	p.Experiment.Kind = "role_change"
	p.Experiment.Candidate.CompiledHash = catalog.CompiledHash(p.Definition)
	return p
}

func TestEvolutionPlanRejectsResearchToolsAndChangedCompiledFlow(t *testing.T) {
	p := evolutionPlanFixture()
	if err := evolutionValidatePlan(p); err != nil {
		t.Fatal(err)
	}
	p.Definition.Nodes[0].Role = "research"
	p.Experiment.Candidate.CompiledHash = catalog.CompiledHash(p.Definition)
	if err := evolutionValidatePlan(p); err == nil {
		t.Fatal("research tools entered no-tool backend")
	}
	p = evolutionPlanFixture()
	p.Definition.Nodes[0].Instruction = "changed after authorization"
	if err := evolutionValidatePlan(p); err == nil {
		t.Fatal("changed compilation accepted")
	}
}

func TestEvolutionNodeInputCannotInheritHostContext(t *testing.T) {
	p := evolutionPlanFixture()
	original := p.Definition.NodeInput(p.Definition.Nodes[0], nil)
	original.Workspace = "production workspace"
	original.ReviewPurpose = "hidden checker"
	original.PlatformEvidence = &workflow.EvidenceReference{}
	if _, err := evolutionNodeInput(p, original, uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("host context accepted")
	}
	original.Workspace, original.ReviewPurpose, original.PlatformEvidence = "", "", nil
	actual, err := evolutionNodeInput(p, original, "run", "node-run")
	if err != nil {
		t.Fatal(err)
	}
	if actual.Input != "case input only" || actual.ProjectID != p.Definition.ProjectID || actual.RunID != "run" || actual.RequestID != p.RequestID {
		t.Fatal("lost real execution scope")
	}
}

func TestEvolutionMemoryBenefitBaselineNeverInjectsMemory(t *testing.T) {
	p := evolutionPlanFixture()
	p.Experiment.Kind, p.Key.Arm = "memory_benefit", "baseline"
	p.Experiment.Baseline = p.Experiment.Candidate
	p.Memories = []workflow.Experience{{Content: []byte(`{"practice":"evidence"}`)}}
	input := p.Definition.NodeInput(p.Definition.Nodes[0], nil)
	if _, err := evolutionNodeInput(p, input, "run", "node"); err == nil {
		t.Fatal("memory-bearing baseline accepted")
	}
	p.Memories = nil
	if _, err := evolutionNodeInput(p, input, "run", "node"); err != nil {
		t.Fatal(err)
	}
}
