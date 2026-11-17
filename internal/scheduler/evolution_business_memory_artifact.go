package scheduler

import (
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/execution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func evolutionBusinessArtifactsTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, run evolutionBusinessRun) (map[string]experience.Artifact, error) {
	rows, err := tx.Query(ctx, `SELECT id,artifact FROM pangu.experience_artifacts WHERE namespace=$1 AND project_id=$2 AND artifact->>'run_id'=$3 ORDER BY id`, s.Namespace, s.ProjectID, run.RunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := map[string]experience.Artifact{}
	for rows.Next() {
		var id string
		var body []byte
		if err = rows.Scan(&id, &body); err != nil {
			return nil, err
		}
		var artifact experience.Artifact
		if json.Unmarshal(body, &artifact) != nil || artifact.ID != id || artifact.RunID != run.RunID || artifact.ProjectID != s.ProjectID || artifact.Generation != 1 || len(artifact.InputHash) != 64 || len(artifact.ExecutionInputHash) != 64 {
			return nil, evolution.ErrChecker
		}
		artifacts[id] = artifact
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for id, artifact := range artifacts {
		err = experience.ValidateArtifact(experience.ArtifactReference{ID: id, Hash: artifact.Hash}, func(key string) (experience.Artifact, error) {
			value, ok := artifacts[key]
			if !ok {
				return experience.Artifact{}, evolution.ErrChecker
			}
			return value, nil
		}, func([]experience.Reference) error { return nil })
		if err != nil {
			return nil, evolutionBusinessMemoryError(err)
		}
	}
	return artifacts, nil
}

func evolutionBusinessAttemptsTx(ctx context.Context, tx pgx.Tx, run evolutionBusinessRun) ([]execution.Attempt, error) {
	rows, err := tx.Query(ctx, `SELECT to_jsonb(a) FROM pangu.node_attempts a WHERE run_id=$1 ORDER BY node_run_id,attempt`, run.RunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attempts []execution.Attempt
	for rows.Next() {
		var body []byte
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		var attempt execution.Attempt
		if json.Unmarshal(body, &attempt) != nil || attempt.RunID != run.RunID || attempt.FinishedAt == nil || attempt.Attempt < 1 {
			return nil, evolution.ErrChecker
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func evolutionBusinessAttemptCoverage(nodes []NodeRun, attempts []execution.Attempt) bool {
	history := map[string]NodeRun{}
	for _, node := range nodes {
		if _, exists := history[node.ID]; exists {
			return false
		}
		history[node.ID] = node
	}
	latest := map[string]execution.Attempt{}
	for _, attempt := range attempts {
		node, ok := history[attempt.NodeRunID]
		if !ok || node.NodeID != attempt.NodeID || attempt.Attempt > node.Attempt {
			return false
		}
		latest[attempt.NodeRunID] = attempt
	}
	if len(latest) != len(nodes) {
		return false
	}
	for _, node := range nodes {
		attempt := latest[node.ID]
		if attempt.Attempt != node.Attempt || attempt.Status != node.Status || attempt.Status != "succeeded" {
			return false
		}
	}
	return true
}
