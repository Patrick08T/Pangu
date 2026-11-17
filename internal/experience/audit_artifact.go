package experience

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s Store) auditArtifactPageTx(ctx context.Context, tx pgx.Tx, scope Scope, b RunBinding, page Page) (ArtifactSummaryPage, error) {
	result := ArtifactSummaryPage{Items: []ArtifactSummary{}}
	aliases := []string{b.ActualRunID, b.RequestID}
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND artifact->>'run_id'=ANY($3::text[])`, scope.Namespace, scope.ProjectID, aliases).Scan(&result.Total)
	if err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND artifact->>'run_id'=ANY($3::text[]) AND id>$4 ORDER BY id LIMIT $5`, scope.Namespace, scope.ProjectID, aliases, page.AfterID, page.Limit+1)
	if err != nil {
		return result, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return result, err
	}
	cache := map[string]ArtifactSummary{}
	for i, id := range ids {
		item, err := s.auditArtifactTx(ctx, tx, scope, id, map[string]bool{}, cache)
		if err != nil {
			return ArtifactSummaryPage{}, err
		}
		if i < page.Limit {
			result.Items = append(result.Items, item)
		}
	}
	result.HasMore = len(ids) > page.Limit
	if result.HasMore {
		result.NextCursor = ids[page.Limit-1]
	}
	return result, nil
}

func (s Store) auditArtifactTx(ctx context.Context, tx pgx.Tx, scope Scope, id string, path map[string]bool, cache map[string]ArtifactSummary) (ArtifactSummary, error) {
	if old, ok := cache[id]; ok {
		return old, nil
	}
	result := ArtifactSummary{ID: id, References: []Reference{}, Parents: []ArtifactReference{}}
	if path[id] {
		result.AuditState = unprovenAudit("dependency_cycle")
		return result, nil
	}
	if len(path) > 1024 || len(cache) > 4096 {
		result.AuditState = unprovenAudit("traversal_limit")
		return result, nil
	}
	artifact, err := auditArtifactRecordTx(ctx, tx, scope, id)
	if errors.Is(err, ErrNotFound) {
		result.AuditState = unprovenAudit("artifact_missing")
		return result, nil
	}
	if errors.Is(err, ErrInvalidated) {
		result.AuditState = unprovenAudit("artifact_corrupt")
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result = artifactSummary(artifact)
	result.AuditState, err = s.auditReferencesTx(ctx, tx, scope, artifact.References)
	if err != nil {
		return ArtifactSummary{}, err
	}
	result.AuditState = mergeAudit(result.AuditState, artifactProofState(artifact))
	path[id] = true
	defer delete(path, id)
	for _, parent := range artifact.Parents {
		ancestor, err := s.auditArtifactTx(ctx, tx, scope, parent.ID, path, cache)
		if err != nil {
			return ArtifactSummary{}, err
		}
		result.AuditState = mergeAudit(result.AuditState, parentAuditState(ancestor, parent.Hash))
	}
	redactArtifactSummary(&result)
	cache[id] = result
	return result, nil
}

func auditArtifactRecordTx(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Artifact, error) {
	var artifact Artifact
	err := scanJSON(tx.QueryRow(ctx, `SELECT jsonb_build_object(
 'id',artifact->'id','project_id',artifact->'project_id','run_id',artifact->'run_id',
 'node_id',artifact->'node_id','kind',artifact->'kind','hash',artifact->'hash',
 'input_hash',artifact->'input_hash','execution_input_hash',artifact->'execution_input_hash',
 'generation',artifact->'generation','proof_version',artifact->'proof_version',
 'references',artifact->'references','parents',artifact->'parents',
 'invalid_reason',CASE WHEN coalesce(artifact->>'invalid_reason','')='' THEN '' ELSE 'artifact_invalidated' END)
 FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND id=$3`, scope.Namespace, scope.ProjectID, id), &artifact)
	if err != nil {
		return Artifact{}, err
	}
	if artifact.ID != id || artifact.ProjectID != scope.ProjectID {
		return Artifact{}, ErrInvalidated
	}
	return artifact, nil
}

func artifactSummary(a Artifact) ArtifactSummary {
	result := ArtifactSummary{ID: a.ID, NodeID: a.NodeID, Kind: a.Kind, OutputHash: a.Hash, InputHash: a.InputHash, ExecutionInputHash: a.ExecutionInputHash, Generation: a.Generation, References: a.References, Parents: a.Parents}
	if result.References == nil {
		result.References = []Reference{}
	}
	if result.Parents == nil {
		result.Parents = []ArtifactReference{}
	}
	return result
}

func artifactProofState(a Artifact) AuditState {
	if a.ProofVersion != 1 || a.Hash == "" || a.InputHash == "" {
		return unprovenAudit("artifact_unproven")
	}
	if a.ExecutionInputHash == "" {
		return unprovenAudit("execution_input_missing")
	}
	if a.InvalidReason != "" {
		return invalidAudit("artifact_invalidated")
	}
	return validAudit()
}

func parentAuditState(parent ArtifactSummary, expected string) AuditState {
	state := parent.AuditState
	switch state.Status {
	case "invalid":
		state.InvalidReason = "ancestor_invalid"
	case "unproven":
		state.InvalidReason = "ancestor_unproven"
	case "valid":
		if expected == "" || parent.OutputHash != expected {
			state = unprovenAudit("ancestor_hash_mismatch")
		}
	}
	return state
}

func redactArtifactSummary(item *ArtifactSummary) {
	if !item.Redacted {
		return
	}
	item.OutputHash, item.InputHash, item.ExecutionInputHash = "", "", ""
	item.References = []Reference{}
	for i := range item.Parents {
		item.Parents[i].Hash = ""
	}
}
