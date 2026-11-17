package scheduler

import (
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func evolutionArm(plan evolution.ExecutionPlan) evaluation.ExperimentArm {
	if plan.Key.Arm == "baseline" {
		return plan.Experiment.Baseline
	}
	return plan.Experiment.Candidate
}

func evolutionValidatePlan(plan evolution.ExecutionPlan) error {
	env, exp := plan.Environment, plan.Experiment
	if !env.Enabled || env.Backend != "eino_json_v1" || env.Currency == "" || env.Hash == "" || env.Hash != exp.EnvironmentHash || env.ProjectID != plan.Definition.ProjectID || exp.LogicalProjectID != env.ProjectID || exp.Purpose != "evaluation" || exp.Generation != plan.Key.Generation {
		return evolution.ErrIsolation
	}
	if plan.Key.Arm != "baseline" && plan.Key.Arm != "candidate" {
		return evolution.ErrInvalid
	}
	if exp.Kind != "role_change" && exp.Kind != "memory_benefit" {
		return evolution.ErrInvalid
	}
	if exp.Kind == "memory_benefit" && plan.Key.Arm == "baseline" && len(plan.Memories) != 0 {
		return evolution.ErrIsolation
	}
	if plan.MaxCalls < 1 || plan.MaxTokens < 1 {
		return evolution.ErrBudget
	}
	if catalog.CompiledHash(plan.Definition) != evolutionArm(plan).CompiledHash {
		return evolution.ErrConflict
	}
	for _, n := range plan.Definition.Nodes {
		if n.Executor != "eino" || n.Role != "" || n.Policy != nil || n.Approval != nil || n.ConnectionVersion != env.ConnectionVersion {
			return evolution.ErrIsolation
		}
	}
	_, err := plan.Definition.Layers()
	return err
}

// The only model-visible structure is NodeInput. FrozenExperiment and checker data stay server-side.
func evolutionNodeInput(plan evolution.ExecutionPlan, input workflow.NodeInput, run, nodeRun string) (workflow.NodeInput, error) {
	if err := evolutionValidatePlan(plan); err != nil {
		return workflow.NodeInput{}, err
	}
	if input.Workspace != "" || input.ProcessScope != "" || input.PlatformEvidence != nil || input.EvidenceRetry != nil || input.ReviewPurpose != "" || input.ReviewFeedback != nil || input.Revision != nil || input.ApprovalID != "" || input.ApprovalRound != 0 {
		return workflow.NodeInput{}, evolution.ErrIsolation
	}
	matched := false
	for _, n := range plan.Definition.Nodes {
		if n.ID == input.Node.ID {
			left, _ := json.Marshal(n)
			right, _ := json.Marshal(input.Node)
			matched = string(left) == string(right)
		}
	}
	if !matched || input.ProjectID != plan.Definition.ProjectID || input.Input != plan.Definition.Input {
		return workflow.NodeInput{}, evolution.ErrConflict
	}
	input.Experiences = plan.Memories
	input.RequestID, input.RunID, input.NodeRunID = plan.RequestID, run, nodeRun
	return input, nil
}
