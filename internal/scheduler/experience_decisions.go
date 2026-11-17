package scheduler

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/Patrick08T/Pangu/internal/approval"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (r *nodeExperience) checkDecisions(ctx context.Context, input workflow.NodeInput) error {
	ids := map[string]string{}
	if input.ApprovalID != "" {
		ids["execution"] = input.ApprovalID
	}
	if input.ReviewFeedback != nil {
		ids["revision"] = input.ReviewFeedback.ApprovalID
	}
	if input.EvidenceRetry != nil {
		ids["evidence"] = input.EvidenceRetry.ApprovalID
	}
	for kind, id := range ids {
		if id == "" {
			return experience.ErrInvalidated
		}
		gate, err := approval.Get(ctx, r.namespace, id)
		if err != nil {
			return err
		}
		generation := input.ApprovalGeneration
		if generation == 0 {
			generation = 1
		}
		if gate.RunID != r.runID || gate.ProjectID != r.scope.ProjectID || gate.OwnerID != r.request.CreatedBy || gate.ApprovalGeneration != generation {
			return experience.ErrInvalidated
		}
		if err = validateExperienceApproval(ctx, r.namespace, id); err != nil {
			return err
		}
		if err = matchExperienceDecision(input, kind, gate); err != nil {
			return err
		}
	}
	return nil
}

func (r *nodeExperience) ValidateApprovals(ctx context.Context) error {
	gates, err := approval.List(ctx, r.namespace, r.runID)
	if err != nil {
		return err
	}
	for _, gate := range gates {
		if err = validateExperienceApproval(ctx, r.namespace, gate.ID); err != nil {
			return err
		}
	}
	return nil
}

func matchExperienceDecision(input workflow.NodeInput, kind string, gate *approval.Gate) error {
	var subject struct {
		Input workflow.NodeInput `json:"node_input"`
	}
	if json.Unmarshal(gate.Subject, &subject) != nil || subject.Input.Node.Approval == nil {
		return experience.ErrInvalidated
	}
	switch kind {
	case "execution":
		if input.Node.Approval == nil || gate.NodeID != input.Node.ID || gate.Round != input.ApprovalRound || (gate.Status != "approved" && gate.Status != "consumed") {
			return experience.ErrInvalidated
		}
	case "revision":
		if gate.Status != "changes_requested" || gate.Review == nil || input.ReviewFeedback == nil || !slices.Contains(subject.Input.Node.Approval.RevisionNodes, input.Node.ID) {
			return experience.ErrInvalidated
		}
		expected := workflow.ReviewFeedback{ApprovalID: gate.ID, NodeID: gate.NodeID, Round: gate.Round + 1, Reason: gate.Reason, Items: gate.Items, PreviousReview: *gate.Review}
		if !sameDecisionPayload(input.ReviewFeedback, expected) {
			return experience.ErrInvalidated
		}
	case "evidence":
		if gate.Purpose != "evidence" || !input.Node.ReadOnly || input.Node.Approval != nil || input.EvidenceRetry == nil || input.EvidenceRetry.Round < 1 || subject.Input.Node.Approval.SourceNode != input.Node.ID {
			return experience.ErrInvalidated
		}
		if gate.Status != "approved" && gate.Status != "consumed" && gate.Status != "changes_requested" {
			return experience.ErrInvalidated
		}
		expected := workflow.EvidenceRetry{ApprovalID: gate.ID, Round: input.EvidenceRetry.Round, Reason: gate.Reason, Items: gate.Items}
		if !sameDecisionPayload(input.EvidenceRetry, expected) {
			return experience.ErrInvalidated
		}
	default:
		return experience.ErrInvalidated
	}
	return nil
}

func sameDecisionPayload(actual, expected any) bool {
	got, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	want, err := json.Marshal(expected)
	return err == nil && string(got) == string(want)
}
