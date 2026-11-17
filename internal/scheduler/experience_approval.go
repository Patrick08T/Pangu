package scheduler

import "github.com/Patrick08T/Pangu/internal/workflow"

func rebindApprovalInput(d workflow.Definition, input workflow.NodeInput, outputs map[string]workflow.NodeOutput, round int) (workflow.NodeInput, error) {
	fresh := d.NodeInput(input.Node, outputs)
	fresh.RequestID, fresh.ApprovalGeneration = input.RequestID, input.ApprovalGeneration
	fresh.Iteration, fresh.ReviewFeedback = input.Iteration, input.ReviewFeedback
	fresh.EvidenceRetry, fresh.ReviewPurpose = input.EvidenceRetry, input.ReviewPurpose
	fresh.ApprovalRound = round
	return d.BindInput(fresh)
}
