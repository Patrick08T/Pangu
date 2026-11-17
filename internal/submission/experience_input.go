package submission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

// ExperienceSelection is the shared logical selection protocol for accepted inputs.
func ExperienceSelection(request string, input workflow.NodeInput, role, cohort string) (experience.SelectionInput, error) {
	input.Experiences = nil
	hash, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return experience.SelectionInput{}, err
	}
	generation, round, revision := int64(input.ApprovalGeneration), 1, int64(input.ApprovalRound)
	if generation == 0 {
		generation = 1
	}
	if revision == 0 {
		revision = 1
	}
	errorText := ""
	if input.Iteration != nil {
		round = input.Iteration.Round
		if input.Iteration.Feedback != nil {
			errorText = input.Iteration.Feedback.Summary
		}
	}
	if input.ReviewFeedback != nil {
		revision = int64(input.ReviewFeedback.Round)
		errorText = input.ReviewFeedback.Reason
	}
	module, _ := input.Parameters["module"].(string)
	text, err := json.Marshal(struct {
		Input      string
		Parameters map[string]any
		Parents    map[string]workflow.NodeOutput
	}{input.Input, input.Parameters, input.Parents})
	if err != nil {
		return experience.SelectionInput{}, err
	}
	return experience.SelectionInput{ProjectID: input.ProjectID, RunID: request, NodeID: input.Node.ID, Round: round, Generation: generation, WatermarkGeneration: 1, ApprovalRevision: revision, BoundInputHash: hash, RoleID: role, RoleVersion: input.Node.AgentVersion, Module: module, Text: string(text), ErrorText: errorText, CanaryScope: cohort}, nil
}

// ExperiencePayload builds the exact bounded payload from a validated frozen usage.
func ExperiencePayload(usage experience.Usage, versions []experience.Version) ([]workflow.Experience, error) {
	if len(usage.References) != len(versions) || len(versions) > 4 {
		return nil, experience.ErrInvalidated
	}
	result := make([]workflow.Experience, 0, len(versions))
	total := 0
	for i, version := range versions {
		ref := usage.References[i]
		body, err := json.Marshal(version.Content)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		if version.ID != ref.VersionID || version.BodyHash != ref.BodyHash || hex.EncodeToString(sum[:]) != ref.BodyHash || ref.DeliveryRevision < 1 || len(body) > 2048 {
			return nil, experience.ErrInvalidated
		}
		total += len(body)
		if total > 8192 {
			return nil, experience.ErrInvalidated
		}
		item := workflow.Experience{Reference: workflow.MemoryReference{VersionID: ref.VersionID, BodyHash: ref.BodyHash, DeliveryRevision: ref.DeliveryRevision}, Kind: version.Content.Kind, Conditions: version.Content.Conditions, Content: body, Reason: usage.MatchReason}
		for _, source := range version.Sources {
			item.Sources = append(item.Sources, workflow.MemorySource{ID: source.ID, Kind: source.Kind, RunID: source.RunID, NodeID: source.NodeID, EvidenceHash: source.Hash})
		}
		result = append(result, item)
	}
	return result, nil
}
