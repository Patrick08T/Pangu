package evolution

import (
	"context"
	"slices"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/jackc/pgx/v5"
)

func (s Service) freezeMemoriesTx(ctx context.Context, tx pgx.Tx, scope Scope, ids []string) ([]workflow.MemoryReference, []workflow.Experience, error) {
	ids = append([]string(nil), ids...)
	slices.Sort(ids)
	refs := []workflow.MemoryReference{}
	bodies := []workflow.Experience{}
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			return nil, nil, ErrInvalid
		}
		v, err := s.Memory.ReadTx(ctx, tx, experience.Scope{Namespace: scope.Namespace, ProjectID: scope.ProjectID, ActorID: scope.ActorID, Action: "read"}, id)
		if err != nil {
			return nil, nil, err
		}
		if (v.Validity != "active" && v.Validity != "superseded") || v.Tombstoned || v.Delivery == "paused" {
			return nil, nil, ErrForbidden
		}
		ref := workflow.MemoryReference{VersionID: v.ID, BodyHash: v.BodyHash, DeliveryRevision: v.DeliveryRevision}
		refs = append(refs, ref)
		body := workflow.Experience{Reference: ref, Kind: v.Content.Kind, Conditions: v.Content.Conditions, Content: jsonBytes(v.Content)}
		for _, source := range v.Sources {
			body.Sources = append(body.Sources, workflow.MemorySource{ID: source.ID, Kind: source.Kind, RunID: source.RunID, NodeID: source.NodeID, EvidenceHash: source.Hash})
		}
		bodies = append(bodies, body)
	}
	return refs, bodies, nil
}

func memoryCombinationHash(refs []workflow.MemoryReference) (string, error) {
	values := make([]experience.Reference, 0, len(refs))
	for _, ref := range refs {
		values = append(values, experience.Reference{VersionID: ref.VersionID, BodyHash: ref.BodyHash, DeliveryRevision: ref.DeliveryRevision})
	}
	return learning.MemoryCombinationHash(values)
}

func (s Service) recheckMemoriesTx(ctx context.Context, tx pgx.Tx, scope Scope, config intentConfiguration) ([]workflow.Experience, error) {
	refs, bodies, err := s.freezeMemoriesTx(ctx, tx, scope, config.Request.MemoryVersionIDs)
	if err != nil {
		return nil, err
	}
	if config.Request.Kind == "memory_benefit" {
		if len(refs) != 1 || len(config.Memories) != 1 || refs[0].VersionID != config.Memories[0].VersionID || refs[0].BodyHash != config.Memories[0].BodyHash {
			return nil, ErrConflict
		}
		return bodies, nil
	}
	if !slices.Equal(refs, config.Memories) {
		return nil, ErrConflict
	}
	return bodies, nil
}
