package scheduler

import (
	"context"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

func (r *nodeExperience) Observe(ctx context.Context, input workflow.NodeInput, out *workflow.NodeOutput) error {
	if out == nil {
		return experience.ErrInvalidated
	}
	input.RunID, input.RequestID = r.runID, r.request.ID
	if out.Skipped && out.Lineage == nil {
		key, _ := nodeRunIdentity(input)
		input.NodeRunID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(r.runID+"/skipped/"+key)).String()
		input.Experiences = nil
		if err := r.checkParents(ctx, input); err != nil {
			return err
		}
		return r.Record(ctx, input, input, out)
	}
	input.NodeRunID = out.RunID
	ref, err := outputReference(*out)
	if err != nil {
		return err
	}
	expected, err := outputArtifact(input, *out)
	if err != nil {
		return err
	}
	artifact, err := r.store.ReadArtifact(ctx, r.scope, ref.ID)
	if err != nil {
		return err
	}
	if artifact.ID != expected.ID || artifact.RunID != r.runID || artifact.NodeID != input.Node.ID || artifact.Hash != ref.Hash || artifact.Hash != expected.Hash || artifact.InputHash != expected.InputHash || len(artifact.ExecutionInputHash) != 64 || artifact.ExecutionInputHash != out.Lineage.ExecutionInputHash {
		return experience.ErrInvalidated
	}
	return nil
}
