package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (r *nodeExperience) checkParents(ctx context.Context, input workflow.NodeInput) error {
	for _, parent := range input.Parents {
		ref, err := outputReference(parent)
		if err != nil {
			return err
		}
		artifact, err := r.store.ReadArtifact(ctx, r.scope, ref.ID)
		if err != nil {
			return err
		}
		if artifact.Hash != ref.Hash || artifact.InputHash != parent.Lineage.InputHash || len(artifact.ExecutionInputHash) != 64 || artifact.ExecutionInputHash != parent.Lineage.ExecutionInputHash {
			return experience.ErrInvalidated
		}
	}
	return nil
}

// Only server-resolved checkout and platform evidence may extend an approved input.
// Both remain included in the separately recorded execution fingerprint.
func checkExecutionInput(prepared, actual workflow.NodeInput) error {
	if prepared.RequestID != actual.RequestID || prepared.ApprovalID != actual.ApprovalID {
		return experience.ErrInvalidated
	}
	actual.Revision, actual.PlatformEvidence = prepared.Revision, prepared.PlatformEvidence
	before, err := workflow.EffectiveInputHash(prepared)
	if err != nil {
		return err
	}
	after, err := workflow.EffectiveInputHash(actual)
	if err != nil {
		return err
	}
	if before != after {
		return experience.ErrInvalidated
	}
	return nil
}

func (r *nodeExperience) Record(ctx context.Context, prepared, actual workflow.NodeInput, out *workflow.NodeOutput) error {
	if err := r.checkDefinition(prepared); err != nil {
		return err
	}
	if prepared.RunID != r.runID || actual.RunID != r.runID || prepared.NodeRunID != actual.NodeRunID {
		return experience.ErrInvalidated
	}
	if err := checkExecutionInput(prepared, actual); err != nil {
		return err
	}
	out.RunID, out.Iteration = prepared.NodeRunID, prepared.Iteration
	artifact, err := outputArtifact(prepared, *out)
	if err != nil {
		return err
	}
	artifact.ExecutionInputHash, err = workflow.EffectiveInputHash(actual)
	if err != nil {
		return err
	}
	if err = r.store.SaveArtifact(ctx, r.scope, artifact); err != nil {
		return err
	}
	out.Lineage = &workflow.OutputLineage{Version: 1, ArtifactID: artifact.ID, InputHash: artifact.InputHash, OutputHash: artifact.Hash, ExecutionInputHash: artifact.ExecutionInputHash}
	return nil
}
