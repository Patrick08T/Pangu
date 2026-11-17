package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Patrick08T/Pangu/internal/evolution"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

type evolutionMemoryTuple struct {
	NodeID                       string
	Round                        int
	Generation, ApprovalRevision int64
	References                   []experience.Reference
}

func evolutionBusinessMemoryTx(ctx context.Context, tx pgx.Tx, s evolution.Scope, run evolutionBusinessRun) (string, string, error) {
	var history struct {
		Nodes []NodeRun `json:"nodes"`
	}
	if json.Unmarshal(run.Summary, &history) != nil || len(history.Nodes) == 0 {
		return "", "", evolution.ErrChecker
	}
	selections, err := evolutionBusinessSelectionsTx(ctx, tx, s, run)
	if err != nil {
		return "", "", err
	}
	artifacts, err := evolutionBusinessArtifactsTx(ctx, tx, s, run)
	if err != nil {
		return "", "", err
	}
	attempts, err := evolutionBusinessAttemptsTx(ctx, tx, run)
	if err != nil {
		return "", "", err
	}
	if !evolutionBusinessAttemptCoverage(history.Nodes, attempts) {
		return "", "", evolution.ErrChecker
	}
	tuples, err := evolutionBusinessMemoryCoverage(run, history.Nodes, selections, artifacts)
	if err != nil {
		return "", "", err
	}
	proof := struct {
		Selections any
		Artifacts  any
		Attempts   any
		Nodes      any
	}{selections, artifacts, attempts, history.Nodes}
	return evolutionHash(tuples), evolutionHash(proof), nil
}

func evolutionBusinessMemoryCoverage(run evolutionBusinessRun, nodes []NodeRun, selections map[string]evolutionMemorySelection, artifacts map[string]experience.Artifact) ([]evolutionMemoryTuple, error) {
	if len(nodes) != len(selections) || len(nodes) != len(artifacts) {
		return nil, evolution.ErrChecker
	}
	visited, keys := map[string]bool{}, map[string]evolutionMemoryTuple{}
	for _, node := range nodes {
		round := node.LoopRound
		if round == 0 {
			round = 1
		}
		key := fmt.Sprintf("%s/%d", node.NodeID, round)
		selection, ok := selections[key]
		if !ok {
			return nil, evolution.ErrChecker
		}
		if _, duplicate := keys[key]; duplicate {
			return nil, evolution.ErrChecker
		}
		artifact, ok := artifacts[node.ID]
		if !ok || artifact.NodeID != node.NodeID || !slices.Equal(artifact.References, selection.Usage.References) {
			return nil, evolution.ErrChecker
		}
		visited[node.NodeID] = true
		input := selection.Usage.Input
		refs := append([]experience.Reference{}, selection.Usage.References...)
		keys[key] = evolutionMemoryTuple{NodeID: node.NodeID, Round: round, Generation: input.Generation, ApprovalRevision: input.ApprovalRevision, References: refs}
	}
	if len(visited) != len(run.Snapshot.Workflow.Nodes) {
		return nil, evolution.ErrChecker
	}
	for _, node := range run.Snapshot.Workflow.Nodes {
		if !visited[node.ID] {
			return nil, evolution.ErrChecker
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	slices.Sort(ordered)
	tuples := make([]evolutionMemoryTuple, 0, len(ordered))
	for _, key := range ordered {
		tuples = append(tuples, keys[key])
	}
	return tuples, nil
}
