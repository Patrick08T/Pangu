package scheduler

import (
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (r *nodeExperience) EvidenceApprovalInput(ctx context.Context, input workflow.NodeInput, out workflow.NodeOutput) (workflow.NodeInput, error) {
	if !input.Node.ReadOnly || input.Node.Approval != nil || out.Skipped || out.Lineage == nil || out.Lineage.ArtifactID != out.RunID {
		return workflow.NodeInput{}, experience.ErrInvalidated
	}
	if err := r.checkDefinition(input); err != nil {
		return workflow.NodeInput{}, err
	}
	if err := r.Observe(ctx, input, &out); err != nil {
		return workflow.NodeInput{}, err
	}
	body, err := json.Marshal(map[string]any{"review": workflow.EvidenceReview(out.Blockers)})
	if err != nil {
		return workflow.NodeInput{}, err
	}
	return workflow.NodeInput{RequestID: input.RequestID, ApprovalGeneration: input.ApprovalGeneration, ApprovalRound: 1, Iteration: input.Iteration, Experiences: input.Experiences, ReviewPurpose: "evidence", ProjectID: input.ProjectID, Input: input.Input, Node: workflow.Node{ID: "evidence-" + out.RunID, Executor: "codex", Instruction: "补证后只读复验 " + input.Node.ID, ReadOnly: true, Approval: &workflow.Approval{SourceNode: input.Node.ID, RevisionNodes: []string{input.Node.ID}}}, Parents: map[string]workflow.NodeOutput{input.Node.ID: {Text: string(body)}}}, nil
}
