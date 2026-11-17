package scheduler

import (
	"sort"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/submission"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func experienceSelection(request string, input workflow.NodeInput, role, cohort string) (experience.SelectionInput, error) {
	return submission.ExperienceSelection(request, input, role, cohort)
}

func experiencePayload(usage experience.Usage, versions []experience.Version) ([]workflow.Experience, error) {
	return submission.ExperiencePayload(usage, versions)
}

func outputArtifact(input workflow.NodeInput, output workflow.NodeOutput) (experience.Artifact, error) {
	if input.RunID == "" || input.NodeRunID == "" || output.RunID != input.NodeRunID {
		return experience.Artifact{}, experience.ErrInvalidated
	}
	inputHash, err := workflow.EffectiveInputHash(input)
	if err != nil {
		return experience.Artifact{}, err
	}
	hash, err := workflow.OutputFingerprint(output)
	if err != nil {
		return experience.Artifact{}, err
	}
	generation := int64(input.ApprovalGeneration)
	if generation == 0 {
		generation = 1
	}
	artifact := experience.Artifact{ID: input.NodeRunID, ProjectID: input.ProjectID, RunID: input.RunID, NodeID: input.Node.ID, Kind: "node_output", Hash: hash, InputHash: inputHash, Generation: generation, ProofVersion: 1, References: memoryReferences(input), Parents: []experience.ArtifactReference{}}
	keys := make([]string, 0, len(input.Parents))
	for key := range input.Parents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ref, err := outputReference(input.Parents[key])
		if err != nil {
			return experience.Artifact{}, err
		}
		artifact.Parents = append(artifact.Parents, ref)
	}
	return artifact, nil
}

func memoryReferences(input workflow.NodeInput) []experience.Reference {
	refs := make([]experience.Reference, 0, len(input.Experiences))
	for _, item := range input.Experiences {
		refs = append(refs, experience.Reference{VersionID: item.Reference.VersionID, BodyHash: item.Reference.BodyHash, DeliveryRevision: item.Reference.DeliveryRevision})
	}
	return refs
}

func outputReference(output workflow.NodeOutput) (experience.ArtifactReference, error) {
	proof := output.Lineage
	if proof == nil || proof.Version != 1 || proof.ArtifactID == "" || len(proof.InputHash) != 64 {
		return experience.ArtifactReference{}, experience.ErrInvalidated
	}
	hash, err := workflow.OutputFingerprint(output)
	if err != nil {
		return experience.ArtifactReference{}, err
	}
	if proof.OutputHash != hash {
		return experience.ArtifactReference{}, experience.ErrInvalidated
	}
	return experience.ArtifactReference{ID: proof.ArtifactID, Hash: hash}, nil
}
