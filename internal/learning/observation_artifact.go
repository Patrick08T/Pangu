package learning

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func observationArtifact(ctx context.Context, tx pgx.Tx, scope experience.Scope, id string) (experience.Artifact, error) {
	var artifact experience.Artifact
	var data []byte
	err := tx.QueryRow(ctx, `SELECT artifact FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id).Scan(&data)
	if err != nil {
		return artifact, err
	}
	if err = json.Unmarshal(data, &artifact); err != nil {
		return artifact, err
	}
	if artifact.ProjectID != scope.ProjectID || artifact.ID != id {
		return artifact, experience.ErrInvalidated
	}
	return artifact, nil
}

func observationReferences(refs []experience.Reference, version experience.Version) error {
	if len(refs) > 1 {
		return experience.ErrInvalidated
	}
	for _, ref := range refs {
		if ref.VersionID != version.ID || ref.BodyHash != version.BodyHash || ref.DeliveryRevision != version.DeliveryRevision {
			return experience.ErrInvalidated
		}
	}
	return nil
}

func observationArtifacts(ctx context.Context, tx pgx.Tx, scope experience.Scope, version experience.Version, runID string, summary []byte, usages []experience.Usage) error {
	var history struct {
		Nodes []struct {
			ID     string `json:"id"`
			NodeID string `json:"node_id"`
		} `json:"nodes"`
	}
	if json.Unmarshal(summary, &history) != nil || len(history.Nodes) == 0 {
		return experience.ErrInvalidated
	}
	consumed := false
	lookup := func(id string) (experience.Artifact, error) { return observationArtifact(ctx, tx, scope, id) }
	check := func(refs []experience.Reference) error { return observationReferences(refs, version) }
	for _, node := range history.Nodes {
		artifact, err := lookup(node.ID)
		if err != nil {
			return err
		}
		if artifact.RunID != runID || artifact.NodeID != node.NodeID || artifact.Kind != "node_output" || artifact.Generation < 1 {
			return experience.ErrInvalidated
		}
		if err = experience.ValidateArtifact(experience.ArtifactReference{ID: node.ID, Hash: artifact.Hash}, lookup, check); err != nil {
			return err
		}
		if len(artifact.References) == 0 {
			continue
		}
		matched := false
		for _, usage := range usages {
			if usage.Input.NodeID == node.NodeID && usage.Input.Generation == artifact.Generation && jsonHash(usage.References) == jsonHash(artifact.References) {
				matched = true
			}
		}
		if !matched {
			return experience.ErrInvalidated
		}
		consumed = true
	}
	if !consumed {
		return experience.ErrInvalidated
	}
	return nil
}
