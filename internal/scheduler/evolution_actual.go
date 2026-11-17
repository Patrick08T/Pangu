package scheduler

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/catalog"
	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/jackc/pgx/v5"
)

// Hash the actual execution artifact, then bind the original role body to that full artifact.
// Expected experiment-arm hashes are deliberately not a measurement source.
func evolutionActualIdentityTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan) (string, string, string, error) {
	memory, err := evolutionActualMemoryHash(p.Memories)
	if err != nil {
		return "", "", "", err
	}
	request, err := evolutionActualRequestTx(ctx, tx, s, p)
	if err != nil {
		return "", "", "", err
	}
	payload, err := evolutionActualPayloadTx(ctx, tx, s, p, request)
	if err != nil {
		return "", "", "", err
	}
	prepared, err := catalog.PrepareExperimentCandidateTx(ctx, tx, s.Namespace, s.ProjectID, request.BaselineVersion, payload)
	if err != nil {
		return "", "", "", err
	}
	definition, role := prepared.Baseline, prepared.BaselineHash
	if p.Key.Arm == "candidate" {
		definition, role = prepared.Candidate, prepared.CandidateHash
	} else if p.Key.Arm != "baseline" {
		return "", "", "", evolution.ErrInvalid
	}
	compiled := catalog.CompiledHash(p.Definition)
	if compiled != catalog.CompiledHash(definition) {
		return "", "", "", evolution.ErrConflict
	}
	return compiled, role, memory, nil
}

func evolutionActualRequestTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan) (evolution.CandidateRequest, error) {
	var raw []byte
	var request evolution.CandidateRequest
	err := tx.QueryRow(ctx, `SELECT request->'request' FROM pangu.evolution_intents WHERE id=$1 AND namespace=$2 AND project_id=$3 AND generation=$4 AND experiment_id=$5 AND experiment_hash=$6`, p.Key.IntentID, s.Namespace, s.ProjectID, p.Key.Generation, p.Experiment.ID, p.Experiment.Hash).Scan(&raw)
	if err != nil {
		return request, err
	}
	if err = json.Unmarshal(raw, &request); err != nil {
		return request, err
	}
	kind := request.Kind
	if kind == "" {
		kind = "role_change"
	}
	if request.BaselineVersion != p.Experiment.BaselineVersion || kind != p.Experiment.Kind {
		return request, evolution.ErrConflict
	}
	return request, nil
}

func evolutionActualPayloadTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, p evolution.ExecutionPlan, request evolution.CandidateRequest) (catalog.CandidatePayload, error) {
	if p.Experiment.Kind == "memory_benefit" {
		base, err := catalog.ReadVersionScopedTx(ctx, tx, s.Namespace, request.BaselineVersion, catalog.VersionScope{ProjectID: s.ProjectID})
		if err != nil {
			return catalog.CandidatePayload{}, err
		}
		if base.Spec.Agent == nil {
			return catalog.CandidatePayload{}, evolution.ErrConflict
		}
		payload := request.Payload
		payload.Agent = *base.Spec.Agent
		return payload, nil
	}
	var raw []byte
	var hash string
	var payload catalog.CandidatePayload
	err := tx.QueryRow(ctx, `SELECT payload,payload_hash FROM pangu.catalog_experiment_candidates WHERE experiment_id=$1 AND payload IS NOT NULL`, p.Experiment.ID).Scan(&raw, &hash)
	if err != nil {
		return payload, err
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	if evolutionHash(payload) != hash {
		return payload, evolution.ErrConflict
	}
	return payload, nil
}
